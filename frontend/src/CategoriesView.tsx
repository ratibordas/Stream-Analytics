import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  CatalogGame, CategoryRow, fetchGames, fetchKeysStatus, fetchQueries, fmtNum,
  loadQueriesFromBrowser, localToUnix, QUERIES_LS, saveQueries, searchGameCatalog, triggerPoll,
} from './api'
import { useI18n } from './i18n'
import { GameIcon } from './Icons'

const POLL_MS = 60_000

export default function GamesView() {
  const { t, lang } = useI18n()
  const [sp, setSp] = useSearchParams()
  const [tracked, setTracked] = useState<string[]>(() => loadQueriesFromBrowser() ?? [])
  const [rows, setRows] = useState<CategoryRow[]>([])
  const [err, setErr] = useState('')
  const [filter, setFilter] = useState('')
  const [sortKey, setSortKey] = useState<keyof CategoryRow>('avg_viewers')
  const [desc, setDesc] = useState(true)
  const [selectorOpen, setSelectorOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  const from = sp.get('from')
  const to = sp.get('to')

  // if the browser has no saved list yet, adopt whatever the server tracks
  useEffect(() => {
    if ((loadQueriesFromBrowser() ?? []).length === 0) {
      fetchQueries().then((r) => r.queries.length && setTracked(r.queries)).catch(() => {})
    }
  }, [])

  const loadRows = useCallback(() => {
    fetchGames('youtube', localToUnix(from), localToUnix(to))
      .then((d) => { setRows(d.items); setErr('') })
      .catch((e) => setErr(String(e)))
  }, [from, to])

  useEffect(() => {
    loadRows()
    const tm = setInterval(loadRows, POLL_MS)
    return () => clearInterval(tm)
  }, [loadRows])

  const applyTracked = async (next: string[], poll: boolean) => {
    const uniq = [...new Set(next.map((s) => s.trim()).filter(Boolean))]
    setTracked(uniq)
    localStorage.setItem(QUERIES_LS, JSON.stringify(uniq))
    setBusy(true)
    setErr('')
    try {
      await saveQueries(uniq)
      if (poll) await triggerPoll()
      loadRows()
    } catch (e) {
      setErr(String(e))
    } finally {
      setBusy(false)
    }
  }

  const trackedSet = new Set(tracked.map((s) => s.toLowerCase()))
  const shown = rows
    .filter((r) => trackedSet.has(r.category.toLowerCase()))
    .filter((r) => r.category.toLowerCase().includes(filter.trim().toLowerCase()))
    .sort((a, b) => {
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
      <div className="filters gamesbar">
        {tracked.map((g) => (
          <span className="chip" key={g}>
            {g}
            <button className="x" onClick={() => applyTracked(tracked.filter((x) => x !== g), false)}
              disabled={busy} aria-label="remove">×</button>
          </span>
        ))}
        <button className="pollbtn" onClick={() => setSelectorOpen((o) => !o)}>+ {t('gAdd')}</button>
        {tracked.length > 0 && (
          <input className="gfilter" placeholder={t('gFilter')} value={filter}
            onChange={(e) => setFilter(e.target.value)} />
        )}
        <label className="dt">
          {t('fPeriodFrom')}{' '}
          <input type="datetime-local" value={from ?? ''} onChange={(e) => setParam('from', e.target.value)} />
        </label>
        <label className="dt">
          {t('fPeriodTo')}{' '}
          <input type="datetime-local" value={to ?? ''} onChange={(e) => setParam('to', e.target.value)} />
        </label>
      </div>

      {selectorOpen && (
        <GameSelector
          tracked={tracked}
          onClose={() => setSelectorOpen(false)}
          onApply={(names) => { setSelectorOpen(false); applyTracked(names, true) }}
        />
      )}

      {err && <div className="meta-line"><span className="err">{err}</span></div>}

      {tracked.length === 0 ? (
        <div className="empty-cta">
          <p>{t('gEmptyTracked')}</p>
          <button className="pollbtn" onClick={() => setSelectorOpen(true)}>{t('gChoose')}</button>
        </div>
      ) : (
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
              {shown.map((r) => (
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
              {shown.length === 0 && (
                <tr><td colSpan={7} className="empty">{busy ? t('loadingMore') : t('emptyPeriod')}</td></tr>
              )}
            </tbody>
          </table>
        </div>
      )}
      <p className="hint">{t('catHint')}</p>
    </>
  )
}

function GameSelector(
  { tracked, onClose, onApply }: { tracked: string[]; onClose: () => void; onApply: (names: string[]) => void },
) {
  const { t } = useI18n()
  const [q, setQ] = useState('')
  const [results, setResults] = useState<CatalogGame[]>([])
  const [checked, setChecked] = useState<Set<string>>(() => new Set(tracked))
  const [loading, setLoading] = useState(false)
  const [rawgOk, setRawgOk] = useState(true)

  useEffect(() => {
    fetchKeysStatus().then((s) => setRawgOk(s.rawg_configured)).catch(() => {})
  }, [])

  useEffect(() => {
    let cancel = false
    setLoading(true)
    const tm = setTimeout(() => {
      searchGameCatalog(q)
        .then((r) => !cancel && setResults(r.items))
        .catch(() => !cancel && setResults([]))
        .finally(() => !cancel && setLoading(false))
    }, 400)
    return () => { cancel = true; clearTimeout(tm) }
  }, [q])

  const toggle = (name: string) => {
    setChecked((prev) => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  const top = [...checked].map((name) => ({ name, image: results.find((r) => r.name === name)?.image ?? '' }))
  const bottom = results.filter((r) => !checked.has(r.name))
  const qv = q.trim()
  const canAddFree =
    qv !== '' &&
    ![...checked].some((n) => n.toLowerCase() === qv.toLowerCase()) &&
    !results.some((r) => r.name.toLowerCase() === qv.toLowerCase())

  const row = (g: CatalogGame, on: boolean) => (
    <label className="selrow" key={g.name}>
      <input type="checkbox" checked={on} onChange={() => toggle(g.name)} />
      <GameIcon url={g.image} name={g.name} />
      <span>{g.name}</span>
    </label>
  )

  return (
    <div className="selector">
      <input autoFocus className="gfilter" placeholder={t('gSearch')} value={q}
        onChange={(e) => setQ(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && canAddFree && (toggle(qv), setQ(''))} />
      {!rawgOk && <p className="hint">{t('gNoRawg')}</p>}
      <div className="selector-list">
        {canAddFree && (
          <button className="selrow addfree" onClick={() => { toggle(qv); setQ('') }}>
            + {t('gAddFree')} “{qv}”
          </button>
        )}
        {top.map((g) => row(g, true))}
        {top.length > 0 && bottom.length > 0 && <div className="seldiv" />}
        {bottom.map((g) => row(g, false))}
        {loading && <div className="empty">{t('loadingMore')}</div>}
      </div>
      <div className="selector-actions">
        <button className="reset" onClick={onClose}>{t('gCancel')}</button>
        <button className="pollbtn" onClick={() => onApply([...checked])}>{t('gApply')}</button>
      </div>
    </div>
  )
}
