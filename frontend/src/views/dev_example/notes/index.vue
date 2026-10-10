<template>
  <div class="dev-example-page">
    <div class="page-header">
      <h2>{{ t('system.notes.title') }}</h2>
      <p class="text-muted">{{ t('system.notes.subtitle') }}</p>
    </div>

    <!-- 操作栏 -->
    <div class="toolbar">
      <el-button type="primary" @click="openCreate" v-permission="'dev_example:notes:create'">
        <el-icon><Plus /></el-icon> {{ t('system.notes.add') }}
      </el-button>
      <el-button @click="fetchList" :loading="loading">
        <el-icon><Refresh /></el-icon> {{ t('common.actions.refresh') }}
      </el-button>
    </div>

    <!-- 列表 -->
    <el-table :data="tableData" v-loading="loading" stripe style="width: 100%; margin-top: 16px">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="title" :label="t('system.notes.titleCol')" min-width="160" />
      <el-table-column prop="content" :label="t('system.notes.content')" min-width="200" show-overflow-tooltip />
      <el-table-column :label="t('system.notes.priority')" width="100">
        <template #default="{ row }">
          <el-tag :type="priorityType(row.priority)" size="small">
            {{ priorityText(row.priority) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.notes.status')" width="80">
        <template #default="{ row }">
          <el-tag :type="row.completed ? 'success' : 'info'" size="small">
            {{ row.completed ? t('system.notes.completed') : t('system.notes.pending') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" :label="t('system.notes.createTime')" width="170" />
      <el-table-column :label="t('system.notes.actions')" width="160" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="openEdit(row)" v-permission="'dev_example:notes:edit'">{{ t('common.actions.edit') }}</el-button>
          <el-button link type="danger" size="small" @click="handleDelete(row)" v-permission="'dev_example:notes:delete'">{{ t('common.actions.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination">
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @size-change="fetchList"
        @current-change="fetchList"
      />
    </div>

    <!-- 新增/编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="editingId ? t('system.notes.edit') : t('system.notes.add')" width="500px">
      <el-form :model="form" label-width="70px">
        <el-form-item :label="t('system.notes.titleCol')" required>
          <el-input v-model="form.title" :placeholder="t('system.notes.titlePlaceholder')" maxlength="200" />
        </el-form-item>
        <el-form-item :label="t('system.notes.content')">
          <el-input v-model="form.content" type="textarea" :rows="4" :placeholder="t('system.notes.contentPlaceholder')" maxlength="5000" />
        </el-form-item>
        <el-form-item :label="t('system.notes.priority')">
          <el-select v-model="form.priority" style="width: 100%">
            <el-option :label="t('system.notes.priorityNormal')" :value="0" />
            <el-option :label="t('system.notes.priorityImportant')" :value="1" />
            <el-option :label="t('system.notes.priorityUrgent')" :value="2" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="editingId" :label="t('system.notes.status')">
          <el-switch v-model="form.completed" :active-text="t('system.notes.completed')" :inactive-text="t('system.notes.pending')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.actions.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.actions.save') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Plus, Refresh } from '@element-plus/icons-vue'
import request from '@/api/request'

const { t } = useI18n()

const loading = ref(false)
const saving = ref(false)
const tableData = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)

const form = reactive({
  title: '',
  content: '',
  priority: 0,
  completed: false,
})

function priorityType(p: number) {
  return p === 2 ? 'danger' : p === 1 ? 'warning' : 'info'
}

function priorityText(p: number) {
  return p === 2 ? t('system.notes.priorityUrgent') : p === 1 ? t('system.notes.priorityImportant') : t('system.notes.priorityNormal')
}

async function fetchList() {
  loading.value = true
  try {
    const res: any = await request.get('/dev-example/notes', {
      params: { page: page.value, page_size: pageSize.value }
    })
    tableData.value = res.data.items || []
    total.value = res.data.total || 0
  } catch {
    // handled by interceptor
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  form.title = ''
  form.content = ''
  form.priority = 0
  form.completed = false
  dialogVisible.value = true
}

function openEdit(row: any) {
  editingId.value = row.id
  form.title = row.title
  form.content = row.content
  form.priority = row.priority
  form.completed = row.completed
  dialogVisible.value = true
}

async function handleSave() {
  if (!form.title.trim()) {
    ElMessage.warning(t('system.notes.titleRequired'))
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await request.put(`/dev-example/notes/${editingId.value}`, {
        title: form.title,
        content: form.content,
        priority: form.priority,
        completed: form.completed,
      })
      ElMessage.success(t('system.notes.updateSuccess'))
    } else {
      await request.post('/dev-example/notes', {
        title: form.title,
        content: form.content,
        priority: form.priority,
      })
      ElMessage.success(t('system.notes.createSuccess'))
    }
    dialogVisible.value = false
    await fetchList()
  } catch {
    // handled by interceptor
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: any) {
  await ElMessageBox.confirm(t('system.notes.confirmDelete', { title: row.title }), t('common.feedback.tip'), { type: 'warning' })
  await request.delete(`/dev-example/notes/${row.id}`)
  ElMessage.success(t('system.notes.deleteSuccess'))
  await fetchList()
}

onMounted(() => {
  fetchList()
})
</script>

<style scoped>
.dev-example-page {
  padding: 20px;
}
.page-header {
  margin-bottom: 16px;
}
.page-header h2 {
  margin: 0 0 4px;
  font-size: 20px;
}
.page-header .text-muted {
  color: #999;
  font-size: 13px;
  margin: 0;
}
.toolbar {
  display: flex;
  gap: 8px;
}
.pagination {
  margin-top: 16px;
  display: flex;
  justify-content: flex-end;
}
</style>