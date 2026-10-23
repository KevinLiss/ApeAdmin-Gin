<template>
  <el-card shadow="never" class="page-card">
    <!-- Toolbar -->
    <div class="toolbar">
      <el-input
        v-model="query.keyword"
        :placeholder="t('system.user.searchPlaceholder')"
        clearable
        style="width: 220px"
        @keyup.enter="fetchData"
      />
      <el-button type="primary" @click="fetchData">
        <el-icon><Search /></el-icon>{{ t('common.actions.query') }}
      </el-button>
    <el-button type="success" @click="openDialog()" v-permission="'system:user:add'">
      <el-icon><Plus /></el-icon>{{ t('common.actions.add') }}
    </el-button>
    <el-button type="warning" @click="openOnline" v-permission="'system:user:list'">
      <el-icon><Monitor /></el-icon>{{ t('system.user.onlineUsers') }}
    </el-button>
    </div>

    <!-- Table -->
    <el-table :data="list" v-loading="loading" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="username" :label="t('system.user.username')" min-width="110" />
      <el-table-column prop="nickname" :label="t('system.user.nickname')" min-width="110" />
      <el-table-column :label="t('system.user.dept')" min-width="120">
        <template #default="{ row }">{{ row.dept?.name || '—' }}</template>
      </el-table-column>
      <el-table-column :label="t('system.user.roles')" min-width="140">
        <template #default="{ row }">
          <el-tag v-for="r in row.roles" :key="r.id" size="small" class="role-tag">{{ r.name }}</el-tag>
          <span v-if="!row.roles?.length">—</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.user.status')" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? t('common.status.enabled') : t('common.status.disabled') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" :label="t('system.user.createTime')" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column :label="t('common.actions.operation')" width="160" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDialog(row)" v-permission="'system:user:edit'">{{ t('common.actions.edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)" v-permission="'system:user:delete'">{{ t('common.actions.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <el-pagination
      v-model:current-page="query.page"
      v-model:page-size="query.page_size"
      :total="total"
      :page-sizes="[10, 20, 50]"
      layout="total, sizes, prev, pager, next, jumper"
      class="pagination"
      @change="fetchData"
    />
  </el-card>

  <!-- Dialog -->
  <el-dialog
    v-model="dialogVisible"
    :title="editingId ? t('system.user.editUser') : t('system.user.addUser')"
    width="480px"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="80px">
      <el-form-item :label="t('system.user.username')" prop="username">
        <el-input v-model="form.username" :disabled="!!editingId" />
      </el-form-item>
      <el-form-item :label="t('system.user.nickname')" prop="nickname">
        <el-input v-model="form.nickname" />
      </el-form-item>
      <el-form-item v-if="!editingId" :label="t('system.user.password')" prop="password">
        <el-input v-model="form.password" type="password" show-password />
      </el-form-item>
      <el-form-item :label="t('system.user.roles')" prop="role_ids">
        <el-select v-model="form.role_ids" multiple :placeholder="t('system.user.selectRole')" style="width: 100%">
          <el-option v-for="r in roleOptions" :key="r.id" :label="r.name" :value="r.id" />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('system.user.status')">
        <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="dialogVisible = false">{{ t('common.actions.cancel') }}</el-button>
      <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.actions.save') }}</el-button>
    </template>
  </el-dialog>

  <!-- 在线用户抽屉 -->
  <el-drawer v-model="onlineVisible" :title="t('system.user.onlineUsers')" size="520px">
    <el-table :data="onlineSessions" v-loading="onlineLoading" size="small">
      <el-table-column prop="username" :label="t('system.user.username')" width="110" />
      <el-table-column prop="ip" label="IP" width="120" />
      <el-table-column :label="t('system.user.lastActive')" width="160">
        <template #default="{ row }">{{ formatTime(row.last_seen) }}</template>
      </el-table-column>
      <el-table-column :label="t('common.actions.operation')" width="80">
        <template #default="{ row }">
          <el-button link type="danger" @click="handleKick(row)" v-permission="'system:user:edit'">
            {{ t('system.user.kick') }}
          </el-button>
        </template>
      </el-table-column>
    </el-table>
    <template #footer>
      <div style="color: #909399; font-size: 12px">{{ t('system.user.onlineTip', { n: onlineSessions.length }) }}</div>
    </template>
  </el-drawer>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { getUsers, createUser, updateUser, deleteUser, getAllRoles, getOnlineUsers, kickUser } from '@/api'

const { t } = useI18n()

interface UserRow {
  id: number
  username: string
  nickname: string
  dept?: { id: number; name: string }
  roles: { id: number; name: string }[]
  status: number
  created_at: string
}

const list = ref<UserRow[]>([])
const total = ref(0)
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const roleOptions = ref<any[]>([])

const query = reactive({ page: 1, page_size: 10, keyword: '' })
const formRef = ref<FormInstance>()
const form = reactive({
  username: '',
  nickname: '',
  password: '',
  role_ids: [] as number[],
  status: 1,
})

const rules: FormRules = {
  username: [{ required: true, message: () => t('system.user.usernameRequired'), trigger: 'blur' }],
  password: [{ required: true, message: () => t('system.user.passwordRule'), trigger: 'blur' }],
}

async function fetchData() {
  loading.value = true
  try {
    const data: any = await getUsers({
      page: query.page,
      page_size: query.page_size,
      keyword: query.keyword || undefined,
    })
    list.value = data.items || []
    total.value = data.total || 0
  } finally {
    loading.value = false
  }
}

async function loadRoles() {
  try {
    const data: any = await getAllRoles()
    roleOptions.value = data || []
  } catch {
    roleOptions.value = []
  }
}

function openDialog(row?: UserRow) {
  editingId.value = row?.id ?? null
  form.username = row?.username ?? ''
  form.nickname = row?.nickname ?? ''
  form.password = ''
  form.role_ids = row?.roles?.map((r) => r.id) ?? []
  form.status = row?.status ?? 1
  dialogVisible.value = true
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      if (editingId.value) {
        await updateUser(editingId.value, {
          nickname: form.nickname,
          status: form.status,
          role_ids: form.role_ids,
        })
        ElMessage.success(t('common.feedback.updateSuccess'))
      } else {
        await createUser({
          username: form.username,
          nickname: form.nickname,
          password: form.password,
          role_ids: form.role_ids,
          status: form.status,
        })
        ElMessage.success(t('common.feedback.createSuccess'))
      }
      dialogVisible.value = false
      fetchData()
    } finally {
      saving.value = false
    }
  })
}

async function handleDelete(row: UserRow) {
  await ElMessageBox.confirm(t('system.user.confirmDelete', { name: row.username }), t('common.feedback.tip'), { type: 'warning' })
  await deleteUser(row.id)
  ElMessage.success(t('common.feedback.deleteSuccess'))
  fetchData()
}

// ─── 在线用户 ───
const onlineVisible = ref(false)
const onlineLoading = ref(false)
const onlineSessions = ref<any[]>([])

async function openOnline() {
  onlineVisible.value = true
  await loadOnline()
}

async function loadOnline() {
  onlineLoading.value = true
  try {
    const data: any = await getOnlineUsers()
    onlineSessions.value = data?.sessions || []
  } finally {
    onlineLoading.value = false
  }
}

async function handleKick(row: any) {
  await ElMessageBox.confirm(t('system.user.confirmKick', { name: row.username }), t('common.feedback.tip'), { type: 'warning' })
  await kickUser(row.user_id)
  ElMessage.success(t('system.user.kicked'))
  loadOnline()
}

function formatTime(time: string) {
  if (!time) return '—'
  return time.replace('T', ' ').slice(0, 19)
}

onMounted(() => {
  fetchData()
  loadRoles()
})
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
.role-tag {
  margin-right: 4px;
}
.pagination {
  margin-top: 14px;
  justify-content: flex-end;
}
</style>