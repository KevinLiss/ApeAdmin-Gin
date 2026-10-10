/**
 * Remote i18n loader: fetches DB-backed translations from the backend
 * and merges them into the vue-i18n instance (overriding built-in dictionaries).
 *
 * API: GET /i18n/messages/:locale -> { namespace: { key: value } }
 * The endpoint is public (no auth required), returns {} when the locale
 * has no DB translations yet.
 */
import axios from 'axios'
import i18n, { getLocale } from '@/locales'

/** Merge DB translations for a given locale into the i18n instance. */
export async function loadI18nFromServer(locale?: string): Promise<void> {
  const target = locale || getLocale()
  try {
    const res = await axios.get(`/api/v1/i18n/messages/${target}`)
    const data = res?.data
    // Support both raw { namespace: {...} } and standard envelope { code, data }
    const payload = data && typeof data === 'object' && 'code' in data ? data.data : data
    if (payload && typeof payload === 'object' && Object.keys(payload).length > 0) {
      i18n.global.mergeLocaleMessage(target, payload)
    }
  } catch {
    // Silent fallback: built-in dictionaries remain authoritative
  }
}
