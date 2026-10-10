import { createI18n } from 'vue-i18n'
import zhCnCommon from './zh-CN/common.json'
import zhCnMenu from './zh-CN/menu.json'
import zhCnAuth from './zh-CN/auth.json'
import zhCnDashboard from './zh-CN/dashboard.json'
import zhCnSystem from './zh-CN/system.json'
import enUsCommon from './en-US/common.json'
import enUsMenu from './en-US/menu.json'
import enUsAuth from './en-US/auth.json'
import enUsDashboard from './en-US/dashboard.json'
import enUsSystem from './en-US/system.json'

export const SUPPORTED_LOCALES = [
  { code: 'zh-CN', label: '简体中文' },
  { code: 'en-US', label: 'English' },
]

export const DEFAULT_LOCALE = 'zh-CN'
const LOCALE_STORAGE_KEY = 'locale'

const messages: Record<string, any> = {
  'zh-CN': {
    common: zhCnCommon,
    menu: zhCnMenu,
    auth: zhCnAuth,
    dashboard: zhCnDashboard,
    system: zhCnSystem,
  },
  'en-US': {
    common: enUsCommon,
    menu: enUsMenu,
    auth: enUsAuth,
    dashboard: enUsDashboard,
    system: enUsSystem,
  },
}

/** Detect locale: localStorage -> browser language -> zh-CN */
export function detectLocale(): string {
  const saved = localStorage.getItem(LOCALE_STORAGE_KEY)
  if (saved && SUPPORTED_LOCALES.some((l) => l.code === saved)) return saved

  const nav = (navigator.language || '').toLowerCase()
  if (nav.startsWith('zh')) return 'zh-CN'
  if (nav.startsWith('en')) return 'en-US'
  return DEFAULT_LOCALE
}

export function getLocale(): string {
  return (i18n.global.locale as any).value ?? (i18n.global.locale as unknown as string)
}

export function setLocale(locale: string): void {
  i18n.global.locale = locale as any
  localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  document.documentElement.setAttribute('lang', locale)
  document.title = i18n.global.t('common.app.title')
}

const i18n = createI18n({
  legacy: false,
  locale: detectLocale(),
  fallbackLocale: DEFAULT_LOCALE,
  messages,
})

// Initial document meta (lang attribute + page title)
const initialLocale = detectLocale()
document.documentElement.setAttribute('lang', initialLocale)
document.title = i18n.global.t('common.app.title')

export default i18n
