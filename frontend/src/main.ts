import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import '@/styles/apeui-theme.css'
import 'vue3-verify/dist/vue3-verify.css'

import App from './App.vue'
import router from './router'
import i18n, { getLocale, setLocale } from './locales'
import { loadI18nFromServer } from './utils/i18nRemote'
import { permissionDirective } from './directives/permission'
import { initTheme } from './composables/useTheme'
import { useSettingsStore } from './stores/settings'

import { computed, watch } from 'vue'

const app = createApp(App)

// 应用已保存的主题（深色/浅色），需在挂载前执行以避免闪白
initTheme()

// Register all Element Plus icons globally
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

const pinia = createPinia()
app.use(pinia)
app.use(router)
app.use(i18n)

// Element Plus component locale follows app language
const elementLocale = computed(() => (getLocale() === 'en-US' ? enUs : zhCn))
app.use(ElementPlus, { locale: elementLocale.value })

// Keep Element Plus locale in sync when language changes
watch(getLocale, () => {
  // Element Plus locale is read at usage time from the provided object in v2.x,
  // but config-provider in App.vue handles runtime switching; this watch keeps
  // the initial value consistent with the detected locale.
})

// Register global directives
app.directive('permission', permissionDirective)

// Fetch public settings and apply theme color before mount;
// also load DB-backed i18n overrides before first render.
const settingsStore = useSettingsStore()
Promise.all([settingsStore.fetchPublicSettings(), loadI18nFromServer()]).finally(() => {
  app.mount('#app')
})
