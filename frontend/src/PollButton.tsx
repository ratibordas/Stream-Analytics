import { useState } from 'react'
import { triggerPoll } from './api'
import { useI18n } from './i18n'

export default function PollButton({ onDone }: { onDone: () => void }) {
  const { t } = useI18n()
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')

  const run = async () => {
    setBusy(true)
    setMsg('')
    try {
      const r = await triggerPoll()
      setMsg(r.ok ? t('polled') : r.errors.join('; '))
    } catch (e) {
      setMsg(String(e))
    } finally {
      setBusy(false)
      onDone()
    }
  }

  return (
    <span className="pollwrap">
      <button className="pollbtn" onClick={run} disabled={busy}>
        {busy ? t('polling') : t('pollNow')}
      </button>
      {msg && <span className="pollmsg">{msg}</span>}
    </span>
  )
}
