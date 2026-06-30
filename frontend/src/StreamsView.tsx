import { useCallback, useEffect, useRef, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  fetchQueries, fetchStreams, fmtNum, localToUnix, QUERIES_LS, saveQueries,
  StreamRow, triggerPoll,
} from './api'
import FilterBar from './FilterBar'
import { useI18n } from './i18n'
import { Avatar, GameIcon } from './Icons'
import PollButton from './PollButton'

const POLL_MS = 60_000
const PAGE = 50
const SHARP = 30

function SortHeader({ id, label, sub }: { id: string; label: string; sub?: string }) {
  const [sp, setSp] = useSearchParams()
  const active = (sp.get('sort') ?? 'current') === id
  const order = active ? (sp.get('order') ?? 'desc') : undefined
  const click = () => {
    const next = new URLSearchParams(sp)
    next.set('sort', id)
    next.set('order', active && order === 'desc' ? 'asc' : 'desc')
    setSp(next, { replace: true })
  }
  return (
    <th className="sortable" onClick={click}>
      {label} {active ? (order === 'desc' ? '▼' : '▲') : '↕'}
      {sub && <small className="sub">{sub}</small>}
    </th>
  )
}

function TrendCell({ v, sharpTip }: { v: number | null; sharpTip: string }) {
  if (v == null) return <td className="n dim">—</td>
  const sharp = Math.abs(v) >= SHARP
  const cls = `n ${v >= 0 ? 'up' : 'down'}${sharp ? ' sharp' : ''}`
  return (
    <td className={cls} title={sharp ? sharpTip : ''}>
      {v >= 0 ? '▲' : '▼'} {Math.abs(v).toFixed(1)}%
    </td>
  )
}

export default function StreamsView({ platform }: { platform: string }) {
  const { t, lang } = useI18n()
  const [sp] = useSearchParams()
  const [rows, setRows] = useState<StreamRow[]>([])
  const [total, setTotal] = useState(0)
  const [err, setErr] = useState('')
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null)
  const loadingRef = useRef(false)

  const game = sp.get('game') ?? ''
  const streamer = sp.get('streamer') ?? ''
  const min = sp.get('min') ?? ''
  const max = sp.get('max') ?? ''
  const from = sp.get('from')
  const to = sp.get('to')
  const sort = sp.get('sort') ?? 'current'
  const order = sp.get('order') ?? 'desc'
  const live = sp.get('live') ?? ''

  const fetchPage = useCallback(
    async (offset: number, replace: boolean) => {
      if (loadingRef.current) return
      loadingRef.current = true
      try {
        const d = await fetchStreams(platform, {
          game, streamer, min, max,
          from: localToUnix(from), to: localToUnix(to),
          sort, order, live, limit: PAGE, offset,
        })
        setTotal(d.total)
        setRows((prev) => (replace ? d.items : [...prev, ...d.items]))
        setErr('')
        setUpdatedAt(new Date())
      } catch (e) {
        setErr(String(e))
      } finally {
        loadingRef.current = false
      }
    },
    [platform, game, streamer, min, max, from, to, sort, order, live],
  )

  // reset to first page whenever the filter/sort changes, then refresh every minute
  useEffect(() => {
    fetchPage(0, true)
    const tm = setInterval(() => fetchPage(0, true), POLL_MS)
    return () => clearInterval(tm)
  }, [fetchPage])

  // infinite scroll: load the next page when the sentinel row scrolls into view
  const sentinelRef = useRef<HTMLTableRowElement>(null)
  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const io = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting && rows.length < total && !loadingRef.current) {
        fetchPage(rows.length, false)
      }
    })
    io.observe(el)
    return () => io.disconnect()
  }, [rows.length, total, fetchPage])

  // when a game isn't tracked yet, the search returns nothing — let the user
  // add it to the tracked list and fetch it immediately from this view.
  const [tracking, setTracking] = useState(false)
  const trackGame = async () => {
    setTracking(true)
    try {
      const cur = (await fetchQueries()).queries
      if (!cur.some((q) => q.toLowerCase() === game.toLowerCase())) {
        const r = await saveQueries([...cur, game])
        localStorage.setItem(QUERIES_LS, JSON.stringify(r.queries))
      }
      await triggerPoll()
      fetchPage(0, true)
    } catch (e) {
      setErr(String(e))
    } finally {
      setTracking(false)
    }
  }

  const exact = (n: number) => Math.round(n).toLocaleString(lang)
  const periodLabel = from ? `${from}${to ? ` — ${to}` : ''}` : t('last24h')

  return (
    <>
      <FilterBar />
      <div className="meta-line">
        {err ? (
          <span className="err">{err}</span>
        ) : (
          <span>
            {total} {t('nStreams')} · {t('periodLabel')} {periodLabel} · {t('updatedAt')}{' '}
            {updatedAt ? updatedAt.toLocaleTimeString(lang) : '…'}
          </span>
        )}
        <PollButton onDone={() => fetchPage(0, true)} />
      </div>
      <div className="table-wrap">
        <table>
          <thead>
            <tr>
              <th>{t('cStream')}</th>
              <th>{t('cStreamer')}</th>
              <th>{t('cGame')}</th>
              <SortHeader id="current" label={t('cNow')} />
              <SortHeader id="period" label={t('cAvgPeriod')} sub={periodLabel} />
              <th>{t('cPeak')}</th>
              <SortHeader id="trend" label={t('cDPeriod')} sub={periodLabel} />
              <th title={t('tipDMonth')}>{t('cDMonth')}</th>
              <th title={t('tipDSubs')}>{t('cDSubs')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id} className={r.is_live ? '' : 'offline'}>
                <td className="title">
                  <a href={r.url} target="_blank" rel="noreferrer" title={r.title}>
                    {r.title || t('untitled')}
                  </a>
                  <div className="badges">
                    {!r.is_live && <span className="badge off">{t('offline')}</span>}
                    {r.language && <span className="badge">{r.language}</span>}
                    {r.subscribers != null && (
                      <span className="badge subs" title={r.subscribers.toLocaleString(lang)}>
                        {fmtNum(r.subscribers)} {t('subsShort')}
                      </span>
                    )}
                    {r.tags &&
                      r.tags.split(',').slice(0, 3).map((tag) => (
                        <span className="badge tag" key={tag}>{tag.trim()}</span>
                      ))}
                  </div>
                </td>
                <td>
                  <span className="who">
                    <Avatar url={r.avatar_url} name={r.streamer} />
                    {r.streamer}
                  </span>
                </td>
                <td>
                  <span className="who">
                    <GameIcon url={r.game_image_url} name={r.game} />
                    {r.game}
                  </span>
                </td>
                <td className="n" title={r.is_live ? exact(r.current_viewers) : ''}>
                  {r.is_live ? fmtNum(r.current_viewers) : '—'}
                </td>
                <td className="n" title={r.samples_period ? exact(r.avg_viewers_period) : ''}>
                  {r.samples_period ? fmtNum(r.avg_viewers_period) : '—'}
                </td>
                <td className="n" title={r.samples_period ? exact(r.peak_viewers_period) : ''}>
                  {r.samples_period ? fmtNum(r.peak_viewers_period) : '—'}
                </td>
                <TrendCell v={r.trend_period_pct} sharpTip={t('tipSharp')} />
                <TrendCell v={r.trend_month_pct} sharpTip={t('tipSharp')} />
                <TrendCell v={r.subs_trend_month_pct} sharpTip={t('tipSharp')} />
              </tr>
            ))}
            {rows.length === 0 && !err && (
              <tr><td colSpan={9} className="empty">
                {t('emptyStreams')}
                {game.trim() && (
                  <div className="track-cta">
                    <button className="pollbtn" onClick={trackGame} disabled={tracking}>
                      {tracking ? t('polling') : `${t('trackGame')} “${game.trim()}”`}
                    </button>
                  </div>
                )}
              </td></tr>
            )}
            {rows.length < total && (
              <tr ref={sentinelRef} className="sentinel">
                <td colSpan={9} className="empty">{t('loadingMore')}</td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <p className="hint">{t('streamsHint')}</p>
    </>
  )
}
