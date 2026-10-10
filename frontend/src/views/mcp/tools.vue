<template>
  <el-card shadow="never" class="page-card">
    <div class="toolbar">
      <el-select v-model="filterCategory" clearable :placeholder="t('system.mcp.tools.allCategories')" style="width:180px" @change="fetchData">
        <el-option v-for="c in categories" :key="c.name" :label="`${c.name} (${c.count})`" :value="c.name" />
      </el-select>
      <el-button type="primary" @click="fetchData">
        <el-icon><Refresh /></el-icon>{{ t('common.actions.refresh') }}
      </el-button>
    </div>

    <el-table :data="filteredTools" v-loading="loading" stripe>
      <el-table-column prop="name" :label="t('system.mcp.tools.name')" width="180" />
      <el-table-column prop="description" :label="t('system.mcp.tools.description')" min-width="200" />
      <el-table-column :label="t('system.mcp.tools.category')" width="120">
        <template #default="{ row }">
          <el-tag size="small" type="info">{{ row.category || 'system' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.mcp.tools.sourcePlugin')" width="130">
        <template #default="{ row }">
          <span v-if="row.plugin_name">{{ row.plugin_name }}</span>
          <span v-else style="color:#909399">{{ t('system.mcp.tools.builtin') }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.mcp.tools.requiredPermissions')" width="160">
        <template #default="{ row }">
          <el-tag v-for="p in row.required_permissions" :key="p" size="small" type="warning" class="param-tag">{{ p }}</el-tag>
          <span v-if="!row.required_permissions?.length" style="color:#909399">{{ t('common.status.none') }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('system.mcp.tools.inputParams')" min-width="180">
        <template #default="{ row }">
          <el-tag v-for="(v, k) in row.input_schema?.properties || {}" :key="k" size="small" class="param-tag">
            {{ k }}:{{ v.type }}
          </el-tag>
          <span v-if="!Object.keys(row.input_schema?.properties || {}).length">{{ t('system.mcp.tools.noParams') }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions.operation')" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" v-permission="'mcp:tools:call'" @click="openCall(row)">{{ t('system.mcp.tools.callTool') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <!-- Call dialog -->
  <el-dialog v-model="callVisible" :title="t('system.mcp.tools.callTool') + ': ' + (currentTool?.name || '')" width="520px">
    <el-form label-width="90px">
      <el-form-item v-for="(v, k) in currentTool?.input_schema?.properties || {}" :key="k" :label="k">
        <el-input v-model="callArgs[k]" :placeholder="t('system.mcp.tools.typeParam', { type: v.type })" />
      </el-form-item>
      <el-form-item v-if="!Object.keys(currentTool?.input_schema?.properties || {}).length" :label="t('common.actions.detail')">
        <span style="color: #909399">{{ t('system.mcp.tools.noParamsTip') }}</span>
      </el-form-item>
    </el-form>
    <div v-if="callResult !== null" class="result-box">
      <pre>{{ JSON.stringify(callResult, null, 2) }}</pre>
    </div>
    <template #footer>
      <el-button @click="callVisible = false">{{ t('common.actions.close') }}</el-button>
      <el-button type="primary" :loading="calling" @click="doCall">{{ t('system.mcp.tools.callTool') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getMcpTools, getMcpToolCategories, callMcpTool } from '@/api'

const { t } = useI18n()

const tools = ref<any[]>([])
const categories = ref<any[]>([])
const filterCategory = ref('')
const loading = ref(false)
const callVisible = ref(false)
const calling = ref(false)
const currentTool = ref<any>(null)
const callArgs = reactive<Record<string, string>>({})
const callResult = ref<any>(null)

const filteredTools = computed(() => {
  if (!filterCategory.value) return tools.value
  return tools.value.filter((t: any) => t.category === filterCategory.value)
})

async function fetchData() {
  loading.value = true
  try {
    const [toolData, catData]: any = await Promise.all([getMcpTools(), getMcpToolCategories()])
    tools.value = toolData || []
    categories.value = catData || []
  } finally {
    loading.value = false
  }
}

function openCall(row: any) {
  currentTool.value = row
  // Reset args
  Object.keys(callArgs).forEach((k) => delete callArgs[k])
  callResult.value = null
  callVisible.value = true
}

async function doCall() {
  if (!currentTool.value) return
  calling.value = true
  try {
    // Convert string args to proper types based on schema
    const typedArgs: Record<string, any> = {}
    const props = currentTool.value.input_schema?.properties || {}
    for (const [k, v] of Object.entries(callArgs)) {
      const schema = props[k] as any
      if (!callArgs[k]) continue
      if (schema?.type === 'number') typedArgs[k] = Number(callArgs[k])
      else if (schema?.type === 'boolean') typedArgs[k] = callArgs[k] === 'true'
      else typedArgs[k] = callArgs[k]
    }
    const data: any = await callMcpTool(currentTool.value.name, typedArgs)
    callResult.value = data?.result
    ElMessage.success(t('common.feedback.callSuccess'))
  } catch {
    // error handled by interceptor
  } finally {
    calling.value = false
  }
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
.param-tag {
  margin-right: 4px;
  margin-bottom: 4px;
}
.result {
  margin-top: 12px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 6px;
  max-height: 200px;
  overflow: auto;
  font-size: 12px;
  font-family: monospace;
  white-space: pre-wrap;
}
</style>