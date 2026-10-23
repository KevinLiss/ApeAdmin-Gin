import axios from 'axios'
import { ElMessage } from 'element-plus'
import i18n, { getLocale } from '@/locales'

const request = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
})

// Request interceptor: attach token + locale
request.interceptors.request.use(
  (config) => {
    const token = localStorage.getItem('apeadmin_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }
    config.headers['Accept-Language'] = getLocale()
    return config
  },
  (error) => Promise.reject(error)
)

// Response interceptor: handle standard envelope { code, msg, data }
// Use raw i18n.global.t so this module works outside component context.
const t = (key: string, named?: Record<string, unknown>) =>
  i18n.global.t(key, named || {})

// ─── refresh token 静默续期 ───────────────────────────────────────────────
// access token 过期（401）时先尝试用 refresh_token 换新（不打扰用户），
// 失败才清除登录态跳登录页。并发 401 只触发一次刷新，其余请求排队等待
// 刷新结果后自动重试。

let refreshing: Promise<string | null> | null = null

async function tryRefreshToken(): Promise<string | null> {
  const refreshToken = localStorage.getItem('apeadmin_refresh_token')
  if (!refreshToken) return null
  try {
    // 直接用 axios（不带拦截器）避免递归 401
    const res = await axios.post(
      '/api/v1/auth/refresh',
      { refresh_token: refreshToken },
      { timeout: 15000, headers: { 'Accept-Language': getLocale() } }
    )
    const data = res.data
    if (data?.code === 200 && data?.data?.access_token) {
      localStorage.setItem('apeadmin_token', data.data.access_token)
      if (data.data.refresh_token) {
        localStorage.setItem('apeadmin_refresh_token', data.data.refresh_token)
      }
      return data.data.access_token as string
    }
    return null
  } catch {
    return null
  }
}

function redirectToLogin() {
  localStorage.removeItem('apeadmin_token')
  localStorage.removeItem('apeadmin_refresh_token')
  // 如果已在登录页，不跳转也不弹消息，由调用方（如 handleLogin）处理错误提示
  if (!window.location.pathname.includes('/login')) {
    ElMessage.error(t('common.feedback.tokenExpired'))
    // 保留当前 admin_path 前缀（可能被用户修改过，不是默认 /admin）
    const currentPath = window.location.pathname
    const loginPath = currentPath.replace(/\/[^/]*$/, '/login') || '/admin/login'
    window.location.href = loginPath
  }
}

request.interceptors.response.use(
  (response) => {
    const res = response.data
    // Standard envelope: code=200 means success
    if (res && typeof res === 'object' && 'code' in res) {
      if (res.code === 200) {
        return res.data
      }
      ElMessage.error(res.msg || t('common.feedback.requestFailed'))
      return Promise.reject(new Error(res.msg || t('common.feedback.requestFailed')))
    }
    return res
  },
  async (error) => {
    const originalConfig = error.config
    if (error.response?.status === 401 && originalConfig) {
      // 登录/刷新接口本身的 401 不做静默续期（凭据已错，刷新无意义）
      const url: string = originalConfig.url || ''
      const isAuthEndpoint = url.includes('/auth/login') || url.includes('/auth/refresh')
      if (!isAuthEndpoint && !originalConfig._retried) {
        // 并发 401 共享同一次刷新
        refreshing = refreshing || tryRefreshToken().finally(() => {
          refreshing = null
        })
        const newToken = await refreshing
        if (newToken) {
          originalConfig._retried = true
          originalConfig.headers = originalConfig.headers || {}
          originalConfig.headers.Authorization = `Bearer ${newToken}`
          return request(originalConfig)
        }
      }
      redirectToLogin()
    } else {
      const msg = error.response?.data?.msg || error.message || t('common.feedback.networkError')
      ElMessage.error(msg)
    }
    return Promise.reject(error)
  }
)

export default request
