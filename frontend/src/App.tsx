import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { fetchKeysStatus, fetchMeta, loadKeysFromBrowser, Meta, saveKeys } from './api'
import CategoriesView from './CategoriesView'
import { useI18n } from './i18n'
import SettingsView from './SettingsView'
import StreamsView from './StreamsView'

export default function App() {
  const { t, lang } = useI18n()
  const TABS = [
    // Twitch is disabled — re-add `{ id: 'twitch', label: 'Twitch' }` here
    // (and re-enable the backend wiring) to bring it back.
    { id: 'youtube', label: 'YouTube' },
    { id: 'categories', label: t('tabCategories') },
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
    if (!keys || (!keys.twitch_client_id && !keys.youtube_api_key)) return
    fetchKeysStatus()
      .then((s) => {
        if (s.mock) return saveKeys(keys, false).then(reloadMeta)
      })
      .catch(() => {})
  }, [reloadMeta])

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
        {tab === 'categories' ? (
          <CategoriesView />
        ) : tab === 'settings' ? (
          <SettingsView onChanged={reloadMeta} />
        ) : (
          <StreamsView platform={tab} key={tab} />
        )}
      </main>
    </div>
  )
}
