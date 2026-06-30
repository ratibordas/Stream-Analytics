import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { CategoryRow, fetchGames, fmtNum, localToUnix } from './api'
import { useI18n } from './i18n'
import { GameIcon } from './Icons'
import PollButton from './PollButton'

const POLL_MS = 60_000

export default function GamesView() {
  const { t, lang } = useI18n()
  const [sp, setSp] = useSearchParams()
  const [rows, setRows] = useState<CategoryRow[]>([])
  const [err, setErr] = useState('')
  const [sortKey, setSortKey] = useState<keyof CategoryRow>('avg_viewers')
  const [desc, setDesc] = useState(true)

  const from = sp.get('from')
  const to = sp.get('to')

  const load = useCallback(() => {
    fetchGames('youtube', localToUnix(from), localToUnix(to))
      .then((d) => { setRows(d.items); setErr('') })
      .catch((e) => setErr(String(e)))
  }, [from, to])

  useEffect(() => {
    load()
    const tm = setInterval(load, POLL_MS)
    return () => clearInterval(tm)
  }, [load])

  const sorted = [...rows].sort((a, b) => {
    const d = Number(a[sortKey]) - Number(b[sortKey])
    return desc ? -d : d
  })

  const header = (key: keyof CategoryRow, label: string, tip = '') => (
    <th
      className="sortable"
      title={tip}
      onClick={() => (key === sortKey ? setDesc(!desc) : (setSortKey(key), setDesc(true)))}
    >
      {label} {sortKey === key ? (desc ? '▼' : '▲') : '↕'}
    </th>
  )

  const setParam = (k: string, v: string) => {
    const next = new URLSearchParams(sp)
    if (v) next.set(k, v)
    else next.delete(k)
    setSp(next, { replace: true })
  }

  const exact = (n: number) => Math.round(n).toLocaleString(lang)

  return (
    <>
      <div className="filters">
        <label className="dt">
          {t('fPeriodFrom')}{' '}
          <input type="datetime-local" value={from ?? ''} onChange={(e) => setParam('from', e.target.value)} />
        </label>
        <label className="dt">
          {t('fPeriodTo')}{' '}
          <input type="datetime-local" value={to ?? ''} onChange={(e) => setParam('to', e.target.value)} />
        </label>
      </div>
      <div className="meta-line">
        {err && <span className="err">{err}</span>}
        <PollButton onDone={load} />
      </div>
      <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t('cGameName')}</th>
            {header('avg_viewers', t('cAvgViewers'))}
            {header('peak_viewers', t('cPeakViewers'))}
            {header('avg_channels', t('cAvgChannels'))}
            {header('viewers_per_channel', t('cVPC'))}
            {header('trend_pct', t('cTrend'))}
            {header('stability_cv_pct', t('cInstability'), t('tipCV'))}
          </tr>
        </thead>
        <tbody>
          {sorted.map((r) => (
            <tr key={r.category}>
              <td>
                <span className="who">
                  <GameIcon url={r.image_url} name={r.category} />
                  {r.category}
                </span>
              </td>
              <td className="n" title={exact(r.avg_viewers)}>{fmtNum(r.avg_viewers)}</td>
              <td className="n" title={exact(r.peak_viewers)}>{fmtNum(r.peak_viewers)}</td>
              <td className="n" title={exact(r.avg_channels)}>{r.avg_channels.toFixed(0)}</td>
              <td className="n strong" title={exact(r.viewers_per_channel)}>{fmtNum(r.viewers_per_channel)}</td>
              <td className={'n ' + (r.trend_pct >= 0 ? 'up' : 'down') + (Math.abs(r.trend_pct) >= 30 ? ' sharp' : '')}>
                {r.trend_pct >= 0 ? '+' : ''}
                {r.trend_pct.toFixed(1)}%
              </td>
              <td className="n" title={t('tipCV')}>{r.stability_cv_pct.toFixed(0)}%</td>
            </tr>
          ))}
          {sorted.length === 0 && !err && (
            <tr><td colSpan={7} className="empty">{t('emptyPeriod')}</td></tr>
          )}
        </tbody>
      </table>
      </div>
      <p className="hint">{t('catHint')}</p>
    </>
  )
}
