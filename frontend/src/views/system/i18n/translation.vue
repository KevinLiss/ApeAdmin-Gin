<template>
  <el-card shadow="never" class="page-card">
    <!-- Toolbar -->
    <div class="toolbar">
      <el-select
        v-model="selectedLocale"
        :placeholder="t('system.i18n.translation.locale')"
        style="width: 180px"
        @change="fetchTranslations"
      >
        <el-option
          v-for="l in locales"
          :key="l.code"
          :label="`${l.icon || ''} ${l.name} (${l.code})`.trim()"
          :value="l.code"
        />
      </el-select>
      <el-select
        v-model="selectedNamespace"
        :placeholder="t('system.i18n.translation.namespace')"
        style="width: 160px"
        @change="fetchTranslations"
      >
        <el-option v-for="ns in namespaces" :key="ns" :label="ns" :value="ns" />
      </el-select>
      <el-input
        v-model="keyword"
        :placeholder="t('system.i18n.translation.searchPlaceholder')"
        clearable
        style="width: 200px"
        @input="applyFilter"
      />
      <el-checkbox v-model="onlyMissing" @change="applyFilter">
        {{ t('system.i18n.translation.onlyMissing') }}
      </el-checkbox>
      <el-button type="primary" :disabled="!selectedLocale" @click="fetchTranslations">
        <el-icon><Refresh /></el-icon>{{ t('common.actions.refresh') }}
      </el-button>
      <el-button
        type="success"
        :disabled="!selectedLocale || !hasChanges"
        :loading="saving"
        @click="handleSave"
      >
        <el-icon><Check /></el-icon>{{ t('system.i18n.translation.batchSave') }}
      </el-button>
      <el-upload
        :show-file-list="false"
        :auto-upload="false"
        accept=".json"
        :on-change="handleImport"
      >
        <el-button :disabled="!selectedLocale">
          <el-icon><Upload /></el-icon>{{ t('system.i18n.translation.importFile') }}
        </el-button>
      </el-upload>
      <el-button :disabled="!selectedLocale" @click="handleExport">
        <el-icon><Download /></el-icon>{{ t('system.i18n.translation.exportFile') }}
      </el-button>
    </div>

    <el-alert
      :title="t('system.i18n.translation.untranslatedTip')"
      type="info"
      :closable="false"
      class="fallback-tip"
    />

    <!-- Table -->
    <el-table :data="filteredRows" v-loading="loading" stripe max-height="620">
      <el-table-column prop="namespace" :label="t('system.i18n.translation.namespace')" width="110" />
      <el-table-column prop="key" :label="t('system.i18n.translation.key')" min-width="200">
        <template #default="{ row }">
          <code class="key-code">{{ row.namespace }}.{{ row.key }}</code>
        </template>
      </el-table-column>
      <el-table-column
        :label="t('system.i18n.translation.sourceValue')"
        min-width="200"
        show-overflow-tooltip
      >
        <template #default="{ row }">
          <span class="source-text">{{ row.sourceValue || '—' }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.i18n.translation.value')" min-width="260">
        <template #default="{ row }">
          <el-input
            v-model="row.value"
            :placeholder="t('system.i18n.translation.untranslated')"
            size="default"
            @input="markChanged(row)"
          />
        </template>
      </el-table-column>
      <el-table-column :label="t('system.i18n.translation.missing')" width="90">
        <template #default="{ row }">
          <el-tag v-if="!row.value" type="warning" size="small">
            {{ t('system.i18n.translation.missing') }}
          </el-tag>
        </template>
      </el-table-column>
    </el-table>

    <div class="summary-bar">
      <span>{{ t('common.pagination.total', { total: filteredRows.length }) }}</span>
      <span v-if="changedKeys.size > 0" class="changed-count">
        {{ changedKeys.size }} ✏️
      </span>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import type { UploadFile } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  getI18nLocalesAll,
  getI18nTranslations,
  saveI18nTranslations,
  exportI18nTranslations,
} from '@/api'
import i18n from '@/locales'

const { t } = useI18n()

interface TransRow {
  namespace: string
  key: string
  value: string
  sourceValue: string
  changed?: boolean
}

const locales = ref<any[]>([])
const selectedLocale = ref('')
const selectedNamespace = ref('')
const keyword = ref('')
const onlyMissing = ref(false)
const loading = ref(false)
const saving = ref(false)
const rows = ref<TransRow[]>([])
const changedKeys = ref(new Set<string>())

// 前端内置命名空间（DB 没有条目时的完整 key 清单来源）
const BUILTIN_NAMESPACES = ['common', 'menu', 'auth', 'dashboard', 'system']

const namespaces = computed(() => {
  const set = new Set<string>(BUILTIN_NAMESPACES)
  rows.value.forEach((r) => set.add(r.namespace))
  return [...set]
})

const filteredRows = computed(() => {
  let result = rows.value
  if (selectedNamespace.value) {
    result = result.filter((r) => r.namespace === selectedNamespace.value)
  }
  if (keyword.value) {
    const kw = keyword.value.toLowerCase()
    result = result.filter((r) => r.key.toLowerCase().includes(kw))
  }
  if (onlyMissing.value) {
    result = result.filter((r) => !r.value)
  }
  return result
})

const hasChanges = computed(() => changedKeys.value.size > 0)

/** Flatten a nested dictionary object into dotted key rows. */
function flattenDict(obj: Record<string, any>, prefix = ''): Array<{ key: string; value: string }> {
  const out: Array<{ key: string; value: string }> = []
  for (const [k, v] of Object.entries(obj || {})) {
    const full = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object') {
      out.push(...flattenDict(v, full))
    } else {
      out.push({ key: full, value: String(v ?? '') })
    }
  }
  return out
}

function markChanged(row: TransRow) {
  row.changed = true
  changedKeys.value.add(`${row.namespace}.${row.key}`)
}

/** Build the full editable list: builtin dict keys as the baseline + DB overrides. */
async function fetchTranslations() {
  if (!selectedLocale.value) return
  loading.value = true
  changedKeys.value = new Set()
  try {
    // 1) Baseline: built-in zh-CN dictionary (the default language) for source display
    const builtinSource = (i18n.global.messages as any)['zh-CN'] || {}
    const sourceFlat = flattenDict(builtinSource)

    // 2) DB translations for the selected locale
    const dbList: any = await getI18nTranslations({
      locale: selectedLocale.value,
      namespace: selectedNamespace.value || '',
    })
    const dbMap = new Map<string, string>()
    for (const item of dbList || []) {
      dbMap.set(`${item.namespace}.${item.key}`, item.value || '')
    }

    // 3) Merge: DB wins over empty; each source key becomes an editable row
    const merged: TransRow[] = sourceFlat.map(({ key, value }) => {
      const ns = key.split('.')[0]
      const bareKey = key.slice(ns.length + 1)
      const dbVal = dbMap.get(key) || ''
      return { namespace: ns, key: bareKey, value: dbVal, sourceValue: value }
    })

    // 4) DB-only keys not in builtin dict (custom keys) get appended
    for (const [fullKey, value] of dbMap) {
      if (!merged.some((r) => `${r.namespace}.${r.key}` === fullKey)) {
        const ns = fullKey.split('.')[0]
        merged.push({
          namespace: ns,
          key: fullKey.slice(ns.length + 1),
          value,
          sourceValue: '',
        })
      }
    }

    rows.value = merged
  } finally {
    loading.value = false
  }
}

async function fetchLocales() {
  const data: any = await getI18nLocalesAll()
  locales.value = data || []
  if (locales.value.length > 0 && !selectedLocale.value) {
    const nonDefault = locales.value.find((l) => l.code !== 'zh-CN')
    selectedLocale.value = nonDefault?.code || locales.value[0].code
  }
}

function applyFilter() {
  // filtering is reactive via computed; nothing to do here
}

async function handleSave() {
  if (changedKeys.value.size === 0) return
  saving.value = true
  try {
    const items = rows.value
      .filter((r) => r.changed)
      .map((r) => ({ namespace: r.namespace, key: r.key, value: r.value || '' }))
    await saveI18nTranslations({ locale_code: selectedLocale.value, items })
    ElMessage.success(t('system.i18n.translation.saveSuccess'))
    changedKeys.value = new Set()
    rows.value.forEach((r) => (r.changed = false))
  } finally {
    saving.value = false
  }
}

async function handleImport(file: UploadFile) {
  if (!file.raw) return
  try {
    const text = await file.raw.text()
    const parsed = JSON.parse(text)
    let items: Array<{ namespace: string; key: string; value: string }> = []

    if (Array.isArray(parsed)) {
      // [{namespace, key, value}]
      items = parsed.map((it: any) => ({
        namespace: it.namespace,
        key: it.key,
        value: String(it.value ?? ''),
      }))
    } else if (parsed && typeof parsed === 'object') {
      // { "namespace.key": "value" } or nested dict
      const flat = flattenDict(parsed)
      items = flat.map(({ key, value }) => {
        const ns = key.split('.')[0]
        return { namespace: ns, key: key.slice(ns.length + 1), value }
      })
    }

    if (items.length === 0) {
      ElMessage.warning(t('common.feedback.requestFailed'))
      return
    }

    await saveI18nTranslations({ locale_code: selectedLocale.value, items })
    ElMessage.success(t('system.i18n.translation.importSuccess', { count: items.length }))
    fetchTranslations()
  } catch {
    ElMessage.error(t('common.feedback.requestFailed'))
  }
}

async function handleExport() {
  try {
    const data: any = await exportI18nTranslations(selectedLocale.value)
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `i18n-${selectedLocale.value}.json`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    // error handled by interceptor
  }
}

onMounted(async () => {
  await fetchLocales()
  if (selectedLocale.value) fetchTranslations()
})
</script>

<style scoped>
.page-card {
  border-radius: 8px;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.fallback-tip {
  margin-bottom: 12px;
}
.key-code {
  font-family: monospace;
  font-size: 12px;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  color: #5a67f5;
  white-space: nowrap;
}
.source-text {
  color: #909399;
  font-size: 13px;
}
.summary-bar {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 12px;
  font-size: 13px;
  color: #909399;
}
.changed-count {
  color: #e6a23c;
  font-weight: 600;
}
</style>

<style>
html.dark .key-code {
  background: #2e3344;
  color: #7f8af8;
}
html.dark .source-text {
  color: #8a90a8;
}
</style>
