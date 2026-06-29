import { useState } from 'react'

function hueFor(name: string): number {
  let h = 0
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) >>> 0
  return h % 360
}

function Fallback({ name, cls }: { name: string; cls: string }) {
  return (
    <span className={cls + ' fallback'} style={{ background: `hsl(${hueFor(name)},45%,38%)` }}>
      {(name[0] || '?').toUpperCase()}
    </span>
  )
}

export function Avatar({ url, name }: { url: string; name: string }) {
  const [err, setErr] = useState(false)
  if (!url || err) return <Fallback name={name} cls="avatar" />
  return <img className="avatar" src={url} onError={() => setErr(true)} alt="" loading="lazy" />
}

export function GameIcon({ url, name }: { url: string; name: string }) {
  const [err, setErr] = useState(false)
  if (!url || err) return <Fallback name={name} cls="gameicon" />
  return <img className="gameicon" src={url} onError={() => setErr(true)} alt="" loading="lazy" />
}
