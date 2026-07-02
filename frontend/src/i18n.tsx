import { createContext, ReactNode, useContext, useState } from 'react'

export type Lang = 'en' | 'ru'
export const LANG_LS = 'sa_lang'

const en = {
  tabGames: 'Games',
  tabSettings: '⚙ Settings',
  mockBadge: 'MOCK data',
  mockTip: 'Add API keys in the Settings tab to get real data',
  dataFrom: 'data as of',
  dataFromTip: 'Time of the last platform poll',
  fGame: 'Game / category…',
  fStreamer: 'Streamer…',
  fMinViewers: 'Viewers from',
  fMaxViewers: 'to',
  fPeriodFrom: 'period from',
  fPeriodTo: 'to',
  fOnlyLive: 'live only',
  fReset: 'Reset',
  cStream: 'Stream',
  cStreamer: 'Streamer',
  cGame: 'Game / category',
  cNow: 'Now',
  cAvgPeriod: 'Avg over period',
  cPeak: 'Peak',
  cDPeriod: 'Δ period',
  cDMonth: 'Δ month',
  cDSubs: 'Δ subs/mo',
  tipDMonth: 'Channel viewer dynamics over the last 30 days',
  tipDSubs: 'Subscriber dynamics over the last 30 days',
  tipSharp: 'Sharp change (≥30%)',
  offline: 'offline',
  subsShort: 'subs',
  untitled: '(untitled)',
  emptyStreams: 'No data yet — the collector is still gathering, or the filter is too strict',
  nStreams: 'streams',
  periodLabel: 'period:',
  last24h: 'last 24h',
  updatedAt: 'updated',
  streamsHint:
    'Δ — change in average viewers: second half of the window vs the first. “Δ period” uses the filtered period, “Δ month” covers the whole channel over 30 days. Sharp changes (≥30%) are highlighted. Subscribers: Twitch does not expose them, the column works for YouTube.',
  pollNow: '⟳ Poll now',
  polling: 'Polling…',
  polled: '✓ polled',
  cCategory: 'Category',
  cAvgViewers: 'Avg viewers',
  cPeakViewers: 'Peak viewers',
  cAvgChannels: 'Avg channels',
  cVPC: 'Viewers per channel',
  cTrend: 'Trend over period',
  cInstability: 'Instability',
  tipCV: 'Coefficient of variation of viewers: lower = more stable',
  emptyPeriod: 'No data for the selected period',
  catHint:
    '“Viewers per channel” is the key profitability metric: how many viewers an average streamer competes for in the category. “Trend” compares the second half of the period with the first. “Instability” is the coefficient of variation: the lower, the steadier the audience. The ideal category: high viewers-per-channel, low instability, trend ≥ 0.',
  sTitle: 'Platform API keys',
  sStorageHint:
    'Keys are stored only in this browser’s localStorage and in the server’s memory — they never end up in the project files. After a server restart the page re-sends them automatically.',
  sClientId: 'Client ID',
  sClientSecret: 'Client Secret',
  sApiKey: 'API Key',
  sWipe: 'wipe collected data on apply (recommended when leaving mock)',
  sApply: 'Validate & apply',
  sApplying: 'Validating keys…',
  sForget: 'Remove from browser',
  sApplied: 'Keys validated and applied — collectors restarted',
  sRemoved: 'Keys removed from the browser (the server keeps using them until restart)',
  sActive: 'active',
  sNone: 'none',
  sLanguage: 'Language',
  qTitle: 'Tracked games',
  qHint:
    'Each entry is a YouTube live-search the collector polls (e.g. “dark souls”, “minecraft”). Stored in this browser and pushed to the server. Searching a game that isn’t tracked offers to add it.',
  qAdd: 'Add a game…',
  qAddBtn: 'Add',
  qApply: 'Save & track',
  qApplied: 'Tracked games updated — collector restarted',
  cGameName: 'Game',
  trackGame: 'Track & fetch',
  loadingMore: 'Loading…',
  gAdd: 'Add games',
  gFilter: 'Filter games…',
  gEmptyTracked: 'No games tracked yet — pick some to follow.',
  gChoose: 'Choose games to track',
  gSearch: 'Search games…',
  gApply: 'Apply',
  gCancel: 'Cancel',
  gAddFree: 'Add',
  gNoRawg: 'Add a RAWG key in Settings for the full game catalog — you can still type a game name here.',
  sRawgHint: 'game catalog',
}

const ru: typeof en = {
  tabGames: 'Игры',
  tabSettings: '⚙ Настройки',
  mockBadge: 'MOCK-данные',
  mockTip: 'Добавь API-ключи во вкладке Настройки, чтобы получать реальные данные',
  dataFrom: 'данные от',
  dataFromTip: 'Время последнего опроса платформ',
  fGame: 'Игра / категория…',
  fStreamer: 'Стример…',
  fMinViewers: 'Зрителей от',
  fMaxViewers: 'до',
  fPeriodFrom: 'период с',
  fPeriodTo: 'по',
  fOnlyLive: 'только live',
  fReset: 'Сбросить',
  cStream: 'Стрим',
  cStreamer: 'Стример',
  cGame: 'Игра / категория',
  cNow: 'Сейчас',
  cAvgPeriod: 'Сред. за период',
  cPeak: 'Пик',
  cDPeriod: 'Δ период',
  cDMonth: 'Δ месяц',
  cDSubs: 'Δ подписчики/мес',
  tipDMonth: 'Динамика зрителей канала за последние 30 дней',
  tipDSubs: 'Динамика подписчиков за последние 30 дней',
  tipSharp: 'Резкое изменение (≥30%)',
  offline: 'офлайн',
  subsShort: 'подп.',
  untitled: '(без названия)',
  emptyStreams: 'Нет данных — коллектор ещё собирает или фильтр слишком строгий',
  nStreams: 'стримов',
  periodLabel: 'период:',
  last24h: 'последние 24ч',
  updatedAt: 'обновлено',
  streamsHint:
    'Δ — изменение средних зрителей: вторая половина окна против первой. «Δ период» — по выбранному фильтром периоду, «Δ месяц» — канал целиком за 30 дней. Резкие изменения (≥30%) подсвечены. Подписчики: Twitch их не отдаёт, колонка работает для YouTube.',
  pollNow: '⟳ Опросить сейчас',
  polling: 'Опрашиваю…',
  polled: '✓ опрошено',
  cCategory: 'Категория',
  cAvgViewers: 'Сред. зрителей',
  cPeakViewers: 'Пик зрителей',
  cAvgChannels: 'Сред. каналов',
  cVPC: 'Зрителей на канал',
  cTrend: 'Тренд за период',
  cInstability: 'Нестабильность',
  tipCV: 'Коэффициент вариации зрителей: меньше = стабильнее',
  emptyPeriod: 'Нет данных за выбранный период',
  catHint:
    '«Зрителей на канал» — главная метрика выгодности: сколько зрителей в среднем приходится на одного стримера в категории. «Тренд» сравнивает вторую половину периода с первой. «Нестабильность» — коэффициент вариации: чем меньше, тем стабильнее аудитория. Идеальная категория: высокие «зрители на канал», низкая нестабильность, тренд ≥ 0.',
  sTitle: 'API-ключи платформ',
  sStorageHint:
    'Ключи хранятся только в localStorage этого браузера и в памяти сервера — в файлы проекта они не попадают. После рестарта сервера страница отправит их повторно сама.',
  sClientId: 'Client ID',
  sClientSecret: 'Client Secret',
  sApiKey: 'API Key',
  sWipe: 'очистить накопленные данные при применении (рекомендуется при уходе с mock)',
  sApply: 'Проверить и применить',
  sApplying: 'Проверяю ключи…',
  sForget: 'Удалить из браузера',
  sApplied: 'Ключи проверены и применены — коллекторы перезапущены',
  sRemoved: 'Ключи удалены из браузера (сервер использует их до рестарта)',
  sActive: 'активен',
  sNone: 'нет',
  sLanguage: 'Язык',
  qTitle: 'Отслеживаемые игры',
  qHint:
    'Каждый пункт — это live-поиск YouTube, который опрашивает коллектор (напр. «dark souls», «minecraft»). Хранится в этом браузере и отправляется на сервер. Поиск игры, которой нет в списке, предложит её добавить.',
  qAdd: 'Добавить игру…',
  qAddBtn: 'Добавить',
  qApply: 'Сохранить и отслеживать',
  qApplied: 'Список игр обновлён — коллектор перезапущен',
  cGameName: 'Игра',
  trackGame: 'Отслеживать и загрузить',
  loadingMore: 'Загрузка…',
  gAdd: 'Добавить игры',
  gFilter: 'Фильтр игр…',
  gEmptyTracked: 'Пока нет отслеживаемых игр — выбери, за какими следить.',
  gChoose: 'Выбрать игры для отслеживания',
  gSearch: 'Поиск игр…',
  gApply: 'Применить',
  gCancel: 'Отмена',
  gAddFree: 'Добавить',
  gNoRawg: 'Добавь ключ RAWG в настройках для полного каталога игр — здесь всё равно можно ввести название вручную.',
  sRawgHint: 'каталог игр',
}

const dict: Record<Lang, typeof en> = { en, ru }

export type TKey = keyof typeof en

interface I18nCtx {
  lang: Lang
  setLang: (l: Lang) => void
  t: (k: TKey) => string
}

const Ctx = createContext<I18nCtx>({ lang: 'en', setLang: () => {}, t: (k) => en[k] })

export function LangProvider({ children }: { children: ReactNode }) {
  const [lang, setLangState] = useState<Lang>(() => {
    const v = localStorage.getItem(LANG_LS)
    return v === 'ru' || v === 'en' ? v : 'en'
  })
  const setLang = (l: Lang) => {
    localStorage.setItem(LANG_LS, l)
    setLangState(l)
  }
  const t = (k: TKey) => dict[lang][k] ?? en[k]
  return <Ctx.Provider value={{ lang, setLang, t }}>{children}</Ctx.Provider>
}

export const useI18n = () => useContext(Ctx)
