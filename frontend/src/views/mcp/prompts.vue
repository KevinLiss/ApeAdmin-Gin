<template>
  <el-card shadow="never" class="page-card">
    <div class="toolbar">
      <el-button type="primary" @click="fetchData">
        <el-icon><Refresh /></el-icon>{{ t('common.actions.refresh') }}
      </el-button>
    </div>

    <el-table :data="prompts" v-loading="loading" stripe>
      <el-table-column prop="name" :label="t('system.mcp.prompts.name')" width="200" />
      <el-table-column prop="description" :label="t('system.mcp.prompts.description')" min-width="260" />
      <el-table-column :label="t('system.mcp.prompts.arguments')" min-width="180">
        <template #default="{ row }">
          <el-tag v-for="arg in row.arguments || []" :key="arg" size="small" class="param-tag">
            {{ arg }}
          </el-tag>
          <span v-if="!(row.arguments || []).length" style="color: #909399">{{ t('system.mcp.prompts.noParams') }}</span>
        </template>
      </el-table-column>
      <el-table-column :label="t('common.actions.operation')" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openRender(row)">{{ t('system.mcp.prompts.renderPrompt') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <!-- Render dialog -->
  <el-dialog v-model="renderVisible" :title="t('system.mcp.prompts.renderTitle', { name: currentPrompt?.name || '' })" width="600px">
    <el-form label-width="90px">
      <el-form-item v-for="arg in currentPrompt?.arguments || []" :key="arg" :label="arg">
        <el-input v-model="renderArgs[arg]" :placeholder="t('system.mcp.prompts.inputArg', { arg })" />
      </el-form-item>
      <el-form-item v-if="!(currentPrompt?.arguments || []).length" :label="t('common.actions.detail')">
        <span style="color: #909399">{{ t('system.mcp.prompts.noParamsTip') }}</span>
      </el-form-item>
    </el-form>
    <div v-if="rendered !== null" class="result-box">
      <pre>{{ rendered }}</pre>
    </div>
    <template #footer>
      <el-button @click="renderVisible = false">{{ t('common.actions.close') }}</el-button>
      <el-button type="primary" :loading="rendering" @click="doRender">{{ t('system.mcp.prompts.renderPrompt') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { getMcpPrompts, renderMcpPrompt } from '@/api'

const { t } = useI18n()

const prompts = ref<any[]>([])
const loading = ref(false)
const renderVisible = ref(false)
const rendering = ref(false)
const currentPrompt = ref<any>(null)
const renderArgs = reactive<Record<string, string>>({})
const rendered = ref<string | null>(null)

async function fetchData() {
  loading.value = true
  try {
    const data: any = await getMcpPrompts()
    prompts.value = data || []
  } finally {
    loading.value = false
  }
}

function openRender(row: any) {
  currentPrompt.value = row
  Object.keys(renderArgs).forEach((k) => delete renderArgs[k])
  rendered.value = null
  renderVisible.value = true
}

async function doRender() {
  if (!currentPrompt.value) return
  rendering.value = true
  try {
    const typedArgs: Record<string, string> = {}
    for (const k of Object.keys(renderArgs)) {
      if (renderArgs[k]) typedArgs[k] = renderArgs[k]
    }
    const data: any = await renderMcpPrompt(currentPrompt.value.name, typedArgs)
    rendered.value = data?.rendered || ''
    ElMessage.success(t('common.feedback.renderSuccess'))
  } catch {
    // error handled by interceptor
  } finally {
    rendering.value = false
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
.result-box {
  margin-top: 12px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 6px;
  max-height: 240px;
  overflow: auto;
  font-size: 12px;
  font-family: monospace;
  white-space: pre-wrap;
}
</style>