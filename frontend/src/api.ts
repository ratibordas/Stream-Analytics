export interface StreamRow {
  platform: string
  id: string
  streamer: string
  title: string
  game: string
  language: string
  url: string
  tags: string
  extra: string
  avatar_url: string
  game_image_url: string
  started_at: number
  last_seen_at: number
  current_viewers: number
  is_live: boolean
  avg_viewers_period: number
  peak_viewers_period: number
  samples_period: number
  trend_period_pct: number | null
  trend_month_pct: number | null
  subs_trend_month_pct: number | null
  subscribers: number | null
}

export interface CategoryRow {
  category: string
  image_url: string
  avg_viewers: number
  peak_viewers: number
  avg_channels: number
  viewers_per_channel: number
  trend_pct: number
  stability_cv_pct: number
  samples: number
}

export interface Meta {
  mock: boolean
  first_snapshot: number
  last_snapshot: number
  snapshot_count: number
  server_time: number
}

async function getJSON<T>(url: string): Promise<T> {
  const r = await fetch(url)
  if (!r.ok) {
    const body = await r.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${r.status}`)
  }
  return r.json()
}

export interface StreamQuery {
  game?: string
  streamer?: string
  min?: string
  max?: string
  from?: number
  to?: number
  sort?: string
  order?: string
  live?: string
}

export function fetchStreams(platform: string, q: StreamQuery) {
  const p = new URLSearchParams()
  if (q.game) p.set('game', q.game)
  if (q.streamer) p.set('streamer', q.streamer)
  if (q.min) p.set('min_viewers', q.min)
  if (q.max) p.set('max_viewers', q.max)
  if (q.from) p.set('from', String(q.from))
  if (q.to) p.set('to', String(q.to))
  if (q.sort) p.set('sort', q.sort)
  if (q.order) p.set('order', q.order)
  if (q.live === '0') p.set('live', '0')
  p.set('limit', '200')
  return getJSON<{ items: StreamRow[]; total: number; from: number; to: number }>(
    `/api/${platform}/streams?${p}`,
  )
}

export function fetchCategories(platform: string, from?: number, to?: number) {
  const p = new URLSearchParams()
  if (from) p.set('from', String(from))
  if (to) p.set('to', String(to))
  return getJSON<{ items: CategoryRow[]; from: number; to: number }>(
    `/api/${platform}/categories?${p}`,
  )
}

export function fetchMeta() {
  return getJSON<Meta>('/api/meta')
}

export async function triggerPoll(): Promise<{ ok: boolean; errors: string[] }> {
  const r = await fetch('/api/poll', { method: 'POST' })
  if (!r.ok) throw new Error(`HTTP ${r.status}`)
  return r.json()
}

export interface ApiKeys {
  twitch_client_id: string
  twitch_client_secret: string
  youtube_api_key: string
}

export interface KeysStatus {
  twitch_configured: boolean
  youtube_configured: boolean
  mock: boolean
}

export const KEYS_LS = 'sa_api_keys'

export function loadKeysFromBrowser(): ApiKeys | null {
  try {
    const raw = localStorage.getItem(KEYS_LS)
    return raw ? (JSON.parse(raw) as ApiKeys) : null
  } catch {
    return null
  }
}

export function fetchKeysStatus() {
  return getJSON<KeysStatus>('/api/keys/status')
}

export async function saveKeys(
  keys: ApiKeys,
  wipeData: boolean,
): Promise<{ ok: boolean; errors: Record<string, string>; status: KeysStatus }> {
  const r = await fetch('/api/keys', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...keys, wipe_data: wipeData }),
  })
  if (!r.ok) {
    const body = await r.json().catch(() => ({}))
    throw new Error(body.error || `HTTP ${r.status}`)
  }
  return r.json()
}

export function fmtNum(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 10_000) return (n / 1000).toFixed(0) + 'k'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(Math.round(n))
}

export function localToUnix(v: string | null): number | undefined {
  if (!v) return undefined
  const t = new Date(v).getTime()
  return Number.isNaN(t) ? undefined : Math.floor(t / 1000)
}
