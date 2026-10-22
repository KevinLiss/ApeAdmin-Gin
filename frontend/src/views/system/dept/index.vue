<template>
  <el-card shadow="never" class="page-card">
    <div class="toolbar">
      <el-button type="success" @click="openDialog()">
        <el-icon><Plus /></el-icon>{{ t('system.dept.addDept') }}
      </el-button>
    </div>

    <el-table
      :data="tree"
      row-key="id"
      v-loading="loading"
      :tree-props="{ children: 'children' }"
      default-expand-all
    >
      <el-table-column prop="name" :label="t('system.dept.name')" min-width="180" />
      <el-table-column prop="leader" :label="t('system.dept.leader')" min-width="100" />
      <el-table-column prop="phone" :label="t('system.dept.phone')" min-width="130" />
      <el-table-column prop="sort" :label="t('system.dept.sort')" width="70" />
      <el-table-column :label="t('system.dept.status')" width="80">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? t('common.status.enabled') : t('common.status.disabled') }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions.operation')" width="160" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openDialog(row)">{{ t('common.actions.edit') }}</el-button>
          <el-button link type="danger" @click="handleDelete(row)">{{ t('common.actions.delete') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <el-dialog v-model="dialogVisible" :title="editingId ? t('system.dept.editDept') : t('system.dept.addDept')" width="480px">
    <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
      <el-form-item :label="t('system.dept.parent')">
        <el-tree-select
          v-model="form.parent_id"
          :data="parentOptions"
          :props="{ value: 'id', label: 'name', children: 'children' }"
          check-strictly
          clearable
          @clear="form.parent_id = 0"
          :placeholder="t('system.dept.parentPlaceholder')"
          style="width: 100%"
        />
      </el-form-item>
      <el-form-item :label="t('system.dept.name')" prop="name">
        <el-input v-model="form.name" />
      </el-form-item>
      <el-form-item :label="t('system.dept.leader')">
        <el-input v-model="form.leader" />
      </el-form-item>
      <el-form-item :label="t('system.dept.phone')">
        <el-input v-model="form.phone" />
      </el-form-item>
      <el-form-item :label="t('system.dept.sort')">
        <el-input-number v-model="form.sort" :min="0" />
      </el-form-item>
      <el-form-item :label="t('system.dept.status')">
        <el-switch v-model="form.status" :active-value="1" :inactive-value="0" />
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
import { getDeptTree, createDept, updateDept, deleteDept } from '@/api'

const { t } = useI18n()

const tree = ref<any[]>([])
const loading = ref(false)
const saving = ref(false)
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const parentOptions = ref<any[]>([])

const formRef = ref<FormInstance>()
const form = reactive({
  parent_id: 0,
  name: '',
  leader: '',
  phone: '',
  sort: 0,
  status: 1,
})

const rules: FormRules = {
  name: [{ required: true, message: () => t('system.dept.nameRequired'), trigger: 'blur' }],
}

async function fetchData() {
  loading.value = true
  try {
    const data: any = await getDeptTree()
    tree.value = buildTree(data || [])
    parentOptions.value = buildParentOptions(editingId.value)
  } finally {
    loading.value = false
  }
}

// buildTree 将后端返回的平铺列表构建为树（孤儿节点自动挂到根，防止 parent 丢失导致部门不显示）
function buildTree(list: any[]): any[] {
  const byId = new Map<number, any>()
  list.forEach((d) => byId.set(d.id, { ...d, children: [] }))
  const roots: any[] = []
  list.forEach((d) => {
    const node = byId.get(d.id)
    const parent = byId.get(d.parent_id ?? 0)
    if (parent && parent !== node) {
      parent.children.push(node)
    } else {
      roots.push(node)
    }
  })
  const sortRec = (nodes: any[]) => {
    nodes.sort((a, b) => (a.sort ?? 0) - (b.sort ?? 0))
    nodes.forEach((n) => sortRec(n.children))
  }
  sortRec(roots)
  return roots
}

// removeBranch 编辑时排除自身及其子孙，防止把上级选成自己的子部门形成环
function removeBranch(nodes: any[], excludedId: number | null): any[] {
  return nodes
    .filter((node) => node.id !== excludedId)
    .map((node) => ({
      ...node,
      children: removeBranch(node.children || [], excludedId),
    }))
}

function buildParentOptions(excludedId: number | null = null) {
  return [{ id: 0, name: t('system.menu.root'), children: removeBranch(tree.value, excludedId) }]
}

function openDialog(row?: any) {
  editingId.value = row?.id ?? null
  form.parent_id = row?.parent_id ?? 0
  form.name = row?.name ?? ''
  form.leader = row?.leader ?? ''
  form.phone = row?.phone ?? ''
  form.sort = row?.sort ?? 0
  form.status = row?.status ?? 1
  parentOptions.value = buildParentOptions(editingId.value)
  dialogVisible.value = true
}

async function handleSave() {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    saving.value = true
    try {
      const payload = {
        parent_id: Number(form.parent_id) || 0,
        name: form.name,
        leader: form.leader,
        phone: form.phone,
        sort: form.sort,
        status: form.status,
      }
      if (editingId.value) {
        await updateDept(editingId.value, payload)
        ElMessage.success(t('common.feedback.updateSuccess'))
      } else {
        await createDept(payload)
        ElMessage.success(t('common.feedback.createSuccess'))
      }
      dialogVisible.value = false
      fetchData()
    } finally {
      saving.value = false
    }
  })
}

async function handleDelete(row: any) {
  await ElMessageBox.confirm(t('system.dept.confirmDelete', { name: row.name }), t('common.feedback.tip'), { type: 'warning' })
  await deleteDept(row.id)
  ElMessage.success(t('common.feedback.deleteSuccess'))
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
</style>