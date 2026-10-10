<template>
  <el-card shadow="never" class="page-card">
    <!-- Toolbar -->
    <div class="toolbar">
      <el-button type="primary" @click="openDialog()" v-permission="'system:i18n:add'">
        <el-icon><Plus /></el-icon>{{ t('system.i18n.locale.add') }}
      </el-button>
      <el-button @click="fetchData">
        <el-icon><Refresh /></el-icon>{{ t('common.actions.refresh') }}
      </el-button>
    </div>

    <!-- Table -->
    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column :label="t('system.i18n.locale.code')" prop="code" width="120">
        <template #default="{ row }">
          <span class="locale-code">{{ row.code }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.i18n.locale.name')" prop="name" min-width="130" />
      <el-table-column :label="t('system.i18n.locale.flag')" prop="icon" width="90">
        <template #default="{ row }">
          <span v-if="row.icon" class="flag-icon">{{ row.icon }}</span>
          <span v-else style="color: #909399">—</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.i18n.locale.isDefault')" width="100">
        <template #default="{ row }">
          <el-tag v-if="row.is_default === 1" type="success" size="small">
            {{ t('system.i18n.locale.defaultTag') }}
          </el-tag>
          <el-button
            v-else
            link
            type="primary"
            size="small"
            v-permission="'system:i18n:edit'"
            @click="handleSetDefault(row)"
          >
            {{ t('system.i18n.locale.setDefault') }}
          </el-button>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.i18n.locale.enabled')" width="90">
        <template #default="{ row }">
          <el-switch
            :model-value="row.is_enabled === 1"
            :disabled="row.is_default === 1"
            @change="handleToggle(row, $event as any)"
          />
        </template>
      </el-table-column>
      <el-table-column prop="sort" :label="t('system.i18n.locale.sort')" width="70" />
      <el-table-column prop="created_at" :label="t('system.i18n.locale.createTime')" width="170" />
      <el-table-column :label="t('common.actions.detail')" width="150" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-permission="'system:i18n:edit'" @click="openDialog(row)">
            {{ t('common.actions.edit') }}
          </el-button>
          <el-button
            link
            type="danger"
            :disabled="row.is_default === 1"
            v-permission="'system:i18n:delete'"
            @click="handleDelete(row)"
          >
            {{ t('common.actions.delete') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <!-- Dialog -->
  <el-dialog
    v-model="dialogVisible"
    :title="editingId ? t('system.i18n.locale.edit') : t('system.i18n.locale.add')"
    width="460px"
    @closed="resetForm"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
      <el-form-item :label="t('system.i18n.locale.code')" prop="code">
        <el-input
          v-model="form.code"
          :disabled="!!editingId"
          :placeholder="t('system.i18n.locale.codePlaceholder')"
        />
      </el-form-item>
      <el-form-item :label="t('system.i18n.locale.name')" prop="name">
        <el-input v-model="form.name" :placeholder="t('system.i18n.locale.namePlaceholder')" />
      </el-form-item>
      <el-form-item :label="t('system.i18n.locale.flag')">
        <el-input v-model="form.icon" placeholder="🇨🇳 / 🇺🇸 / 🇯🇵" maxlength="8" />
      </el-form-item>
      <el-form-item :label="t('system.i18n.locale.isDefault')">
        <el-switch v-model="form.is_default" :active-value="1" :inactive-value="0" />
      </el-form-item>
      <el-form-item :label="t('system.i18n.locale.enabled')">
        <el-switch v-model="form.is_enabled" :active-value="1" :inactive-value="0" />
      </el-form-item>
      <el-form-item :label="t('system.i18n.locale.sort')">
        <el-input-number v-model="form.sort" :min="0" :max="999" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">{{ t('common.actions.cancel') }}</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.actions.save') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import {
  getI18nLocalesAll,
  createI18nLocale,
  updateI18nLocale,
  deleteI18nLocale,
} from '@/api'

const { t } = useI18n()

const list = ref<any[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const formRef = ref<FormInstance>()

const form = reactive({
  code: '',
  name: '',
  icon: '',
  is_enabled: 1,
  is_default: 0,
  sort: 0,
})

const rules = reactive<FormRules>({
  code: [
    { required: true, message: () => t('system.i18n.locale.codeRequired'), trigger: 'blur' },
    {
      pattern: /^[a-z]{2}(-[A-Z]{2})?$/,
      message: () => t('system.i18n.locale.codeFormat'),
      trigger: 'blur',
    },
  ],
  name: [{ required: true, message: () => t('system.i18n.locale.nameRequired'), trigger: 'blur' }],
})

async function fetchData() {
  loading.value = true
  try {
    const data: any = await getI18nLocalesAll()
    list.value = data || []
  } finally {
    loading.value = false
  }
}

function resetForm() {
  form.code = ''
  form.name = ''
  form.icon = ''
  form.is_enabled = 1
  form.is_default = 0
  form.sort = 0
  editingId.value = null
  formRef.value?.resetFields()
}

function openDialog(row?: any) {
  resetForm()
  if (row) {
    editingId.value = row.id
    form.code = row.code
    form.name = row.name
    form.icon = row.icon || ''
    form.is_enabled = row.is_enabled
    form.is_default = row.is_default
    form.sort = row.sort
  }
  dialogVisible.value = true
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      if (editingId.value) {
        await updateI18nLocale(editingId.value, {
          code: form.code,
          name: form.name,
          icon: form.icon,
          is_enabled: form.is_enabled,
          is_default: form.is_default,
          sort: form.sort,
        })
      } else {
        await createI18nLocale({
          code: form.code,
          name: form.name,
          icon: form.icon,
          is_enabled: form.is_enabled,
          is_default: form.is_default,
          sort: form.sort,
        })
      }
      ElMessage.success(t('common.feedback.operationSuccess'))
      dialogVisible.value = false
      fetchData()
    } finally {
      saving.value = false
    }
  })
}

async function handleDelete(row: any) {
  await ElMessageBox.confirm(
    t('system.i18n.locale.confirmDelete', { name: row.name }),
    t('common.feedback.tip'),
    { type: 'warning' }
  )
  await deleteI18nLocale(row.code)
  ElMessage.success(t('common.feedback.deleteSuccess'))
  fetchData()
}

async function handleSetDefault(row: any) {
  await updateI18nLocale(row.id, {
    code: row.code,
    name: row.name,
    icon: row.icon || '',
    is_enabled: row.is_enabled,
    is_default: 1,
    sort: row.sort,
  })
  ElMessage.success(t('common.feedback.operationSuccess'))
  fetchData()
}

async function handleToggle(row: any, enabled: boolean) {
  if (row.is_default === 1) {
    ElMessage.warning(t('system.i18n.locale.cannotDisableDefault'))
    return
  }
  await updateI18nLocale(row.id, {
    code: row.code,
    name: row.name,
    icon: row.icon || '',
    is_enabled: enabled ? 1 : 0,
    is_default: row.is_default,
    sort: row.sort,
  })
  ElMessage.success(t('common.feedback.operationSuccess'))
  fetchData()
}

onMounted(fetchData)
</script>

<style scoped>
.page-card {
  border-radius: 8px;
}
.toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
}
.locale-code {
  font-family: monospace;
  font-size: 12px;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  color: #5a67f5;
}
.flag-icon {
  font-size: 16px;
}
</style>

<style>
html.dark .locale-code {
  background: #2e3344;
  color: #7f8af8;
}
</style>
