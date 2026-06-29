import { useEffect, useState } from 'react'
import {
  ApiKeys, fetchKeysStatus, KEYS_LS, KeysStatus, loadKeysFromBrowser, saveKeys,
} from './api'
import { Lang, useI18n } from './i18n'

const EMPTY: ApiKeys = { twitch_client_id: '', twitch_client_secret: '', youtube_api_key: '' }

export default function SettingsView({ onChanged }: { onChanged: () => void }) {
  const { t, lang, setLang } = useI18n()
  const [keys, setKeys] = useState<ApiKeys>(() => loadKeysFromBrowser() ?? EMPTY)
  const [status, setStatus] = useState<KeysStatus | null>(null)
  const [wipe, setWipe] = useState(false)
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [okMsg, setOkMsg] = useState('')

  useEffect(() => {
    fetchKeysStatus()
      .then((s) => {
        setStatus(s)
        setWipe(s.mock)
      })
      .catch(() => {})
  }, [])

  const set = (k: keyof ApiKeys) => (e: React.ChangeEvent<HTMLInputElement>) =>
    setKeys({ ...keys, [k]: e.target.value.trim() })

  const submit = async () => {
    setBusy(true)
    setErrors({})
    setOkMsg('')
    try {
      const r = await saveKeys(keys, wipe)
      setErrors(r.errors)
      setStatus(r.status)
      if (r.ok) {
        localStorage.setItem(KEYS_LS, JSON.stringify(keys))
        setOkMsg(t('sApplied'))
        onChanged()
      }
    } catch (e) {
      setErrors({ global: String(e) })
    } finally {
      setBusy(false)
    }
  }

  const forget = () => {
    localStorage.removeItem(KEYS_LS)
    setKeys(EMPTY)
    setOkMsg(t('sRemoved'))
  }

  const badge = (on: boolean, label: string) => (
    <span className={'badge ' + (on ? 'subs' : 'off')}>
      {label}: {on ? t('sActive') : t('sNone')}
    </span>
  )

  return (
    <div className="settings">
      <h2>{t('sLanguage')}</h2>
      <div className="tabs small">
        {(['en', 'ru'] as Lang[]).map((l) => (
          <button key={l} className={lang === l ? 'active' : ''} onClick={() => setLang(l)}>
            {l === 'en' ? 'English' : 'Русский'}
          </button>
        ))}
      </div>

      <h2>{t('sTitle')}</h2>
      {status && (
        <p>
          {badge(status.youtube_configured, 'YouTube')}{' '}
          {status.mock && <span className="badge mock">MOCK</span>}
        </p>
      )}
      <p className="hint">{t('sStorageHint')}</p>

      {/* Twitch is disabled. Re-enable by uncommenting this fieldset (and the
          backend wiring + the Twitch tab in App.tsx).
      <fieldset>
        <legend>Twitch — <a href="https://dev.twitch.tv/console/apps" target="_blank" rel="noreferrer">dev.twitch.tv/console/apps</a></legend>
        <label>
          {t('sClientId')}
          <input value={keys.twitch_client_id} onChange={set('twitch_client_id')} autoComplete="off" />
        </label>
        <label>
          {t('sClientSecret')}
          <input type="password" value={keys.twitch_client_secret} onChange={set('twitch_client_secret')} autoComplete="off" />
        </label>
        {errors.twitch && <div className="err">{errors.twitch}</div>}
      </fieldset>
      */}

      <fieldset>
        <legend>YouTube — <a href="https://console.cloud.google.com" target="_blank" rel="noreferrer">console.cloud.google.com</a> (YouTube Data API v3)</legend>
        <label>
          {t('sApiKey')}
          <input type="password" value={keys.youtube_api_key} onChange={set('youtube_api_key')} autoComplete="off" />
        </label>
        {errors.youtube && <div className="err">{errors.youtube}</div>}
      </fieldset>

      <label className="chk">
        <input type="checkbox" checked={wipe} onChange={(e) => setWipe(e.target.checked)} />
        {t('sWipe')}
      </label>

      <div className="settings-actions">
        <button className="pollbtn" onClick={submit} disabled={busy}>
          {busy ? t('sApplying') : t('sApply')}
        </button>
        <button className="reset" onClick={forget}>{t('sForget')}</button>
      </div>
      {errors.global && <div className="err">{errors.global}</div>}
      {okMsg && <div className="ok">{okMsg}</div>}
    </div>
  )
}
