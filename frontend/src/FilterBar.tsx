import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useI18n } from './i18n'

const TEXT_KEYS = ['game', 'streamer', 'min', 'max'] as const

export default function FilterBar({ showDates = true }: { showDates?: boolean }) {
  const { t } = useI18n()
  const [sp, setSp] = useSearchParams()
  const [draft, setDraft] = useState(() =>
    Object.fromEntries(TEXT_KEYS.map((k) => [k, sp.get(k) ?? ''])),
  )

  useEffect(() => {
    setDraft(Object.fromEntries(TEXT_KEYS.map((k) => [k, sp.get(k) ?? ''])))
  }, [sp])

  useEffect(() => {
    const tm = setTimeout(() => {
      let changed = false
      const next = new URLSearchParams(sp)
      for (const k of TEXT_KEYS) {
        const v = draft[k].trim()
        if ((sp.get(k) ?? '') !== v) {
          changed = true
          if (v) next.set(k, v)
          else next.delete(k)
        }
      }
      if (changed) setSp(next, { replace: true })
    }, 800)
    return () => clearTimeout(tm)
  }, [draft])

  const setParam = (k: string, v: string) => {
    const next = new URLSearchParams(sp)
    if (v) next.set(k, v)
    else next.delete(k)
    setSp(next, { replace: true })
  }

  const reset = () => {
    const next = new URLSearchParams()
    const tab = sp.get('tab')
    if (tab) next.set('tab', tab)
    setSp(next)
  }

  const hasFilters = ['game', 'streamer', 'min', 'max', 'from', 'to', 'live', 'sort', 'order']
    .some((k) => sp.get(k))

  return (
    <div className="filters">
      <input
        placeholder={t('fGame')}
        value={draft.game}
        onChange={(e) => setDraft({ ...draft, game: e.target.value })}
      />
      <input
        placeholder={t('fStreamer')}
        value={draft.streamer}
        onChange={(e) => setDraft({ ...draft, streamer: e.target.value })}
      />
      <input
        placeholder={t('fMinViewers')}
        type="number"
        min="0"
        className="num"
        value={draft.min}
        onChange={(e) => setDraft({ ...draft, min: e.target.value })}
      />
      <input
        placeholder={t('fMaxViewers')}
        type="number"
        min="0"
        className="num"
        value={draft.max}
        onChange={(e) => setDraft({ ...draft, max: e.target.value })}
      />
      {showDates && (
        <>
          <label className="dt">
            {t('fPeriodFrom')}{' '}
            <input
              type="datetime-local"
              value={sp.get('from') ?? ''}
              onChange={(e) => setParam('from', e.target.value)}
            />
          </label>
          <label className="dt">
            {t('fPeriodTo')}{' '}
            <input
              type="datetime-local"
              value={sp.get('to') ?? ''}
              onChange={(e) => setParam('to', e.target.value)}
            />
          </label>
        </>
      )}
      <label className="switch">
        <input
          type="checkbox"
          checked={sp.get('live') !== '0'}
          onChange={(e) => setParam('live', e.target.checked ? '' : '0')}
        />
        <span className="slider" />
        {t('fOnlyLive')}
      </label>
      <button onClick={reset} disabled={!hasFilters} className="reset">
        {t('fReset')}
      </button>
    </div>
  )
}
