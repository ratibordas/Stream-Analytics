import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import {
  fetchKeysStatus, fetchMeta, loadKeysFromBrowser, loadQueriesFromBrowser,
  Meta, saveKeys, saveQueries,
} from './api'
import GamesView from './CategoriesView'
import { useI18n } from './i18n'
import SettingsView from './SettingsView'
import StreamsView from './StreamsView'

export default function App() {
  const { t, lang } = useI18n()
  const TABS = [
    // Twitch is disabled — re-add `{ id: 'twitch', label: 'Twitch' }` here
    // (and re-enable the backend wiring) to bring it back.
    { id: 'youtube', label: 'YouTube' },
    { id: 'games', label: t('tabGames') },
    { id: 'settings', label: t('tabSettings') },
  ]
  const [sp, setSp] = useSearchParams()
  const tab = sp.get('tab') ?? 'youtube'
  const [meta, setMeta] = useState<Meta | null>(null)

  const reloadMeta = useCallback(() => {
    fetchMeta().then(setMeta).catch(() => {})
  }, [])

  useEffect(() => {
    reloadMeta()
    const t = setInterval(reloadMeta, 60_000)
    return () => clearInterval(t)
  }, [reloadMeta])

  useEffect(() => {
    const keys = loadKeysFromBrowser()
    if (!keys) return
    fetchKeysStatus()
      .then((s) => {
        const missing =
          s.mock ||
          (!!keys.youtube_api_key && !s.youtube_configured) ||
          (!!keys.rawg_api_key && !s.rawg_configured)
        if (missing) return saveKeys(keys, false).then(reloadMeta)
      })
      .catch(() => {})
  }, [reloadMeta])

  useEffect(() => {
    const queries = loadQueriesFromBrowser()
    if (queries && queries.length) saveQueries(queries).catch(() => {})
  }, [])

  const switchTab = (id: string) => {
    const next = new URLSearchParams(sp)
    next.set('tab', id)
    setSp(next)
  }

  return (
    <div className="app">
      <header>
        <h1>Stream Analytics</h1>
        {meta?.mock && (
          <span className="badge mock" title={t('mockTip')}>
            {t('mockBadge')}
          </span>
        )}
        {meta && meta.last_snapshot > 0 && (
          <span className="badge" title={t('dataFromTip')}>
            {t('dataFrom')} {new Date(meta.last_snapshot * 1000).toLocaleString(lang)}
          </span>
        )}
        <nav className="tabs">
          {TABS.map((t) => (
            <button key={t.id} className={tab === t.id ? 'active' : ''} onClick={() => switchTab(t.id)}>
              {t.label}
            </button>
          ))}
        </nav>
      </header>
      <main>
        {tab === 'games' ? (
          <GamesView />
        ) : tab === 'settings' ? (
          <SettingsView onChanged={reloadMeta} />
        ) : (
          <StreamsView platform={tab} key={tab} />
        )}
      </main>
    </div>
  )
}
