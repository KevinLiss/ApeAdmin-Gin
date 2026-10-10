<template>
  <el-card shadow="never" class="page-card">
    <div class="toolbar">
      <el-button type="primary" @click="fetchData">
        <el-icon><Refresh /></el-icon>{{ t('common.actions.refresh') }}
      </el-button>
    </div>

    <el-table :data="resources" v-loading="loading" stripe>
      <el-table-column prop="name" :label="t('system.mcp.resources.name')" width="200" />
      <el-table-column prop="uri" label="URI" min-width="220">
        <template #default="{ row }">
          <code class="uri-code">{{ row.uri }}</code>
        </template>
      </el-table-column>
      <el-table-column prop="description" :label="t('system.mcp.resources.description')" min-width="250" />
      <el-table-column :label="t('common.actions.operation')" width="120" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="openRead(row)">{{ t('system.mcp.resources.readResource') }}</el-button>
        </template>
      </el-table-column>
    </el-table>
  </el-card>

  <!-- Read dialog -->
  <el-dialog v-model="readVisible" :title="t('system.mcp.resources.readTitle', { name: currentResource?.name || '' })" width="640px">
    <div class="result-box">
      <pre>{{ readResult }}</pre>
    </div>
    <template #footer>
      <el-button @click="readVisible = false">{{ t('common.actions.close') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { getMcpResources, getMcpResourcesRead } from '@/api'

const { t } = useI18n()

const resources = ref<any[]>([])
const loading = ref(false)
const readVisible = ref(false)
const currentResource = ref<any>(null)
const readResult = ref('')

async function fetchData() {
  loading.value = true
  try {
    const data: any = await getMcpResources()
    resources.value = data || []
  } finally {
    loading.value = false
  }
}

async function openRead(row: any) {
  currentResource.value = row
  readResult.value = ''
  readVisible.value = true
  try {
    // GET /mcp/resources/read?uri=xxx
    const params = { uri: row.uri }
    const resp: any = await getMcpResourcesRead(params)
    readResult.value = JSON.stringify(resp, null, 2)
  } catch {
    readResult.value = t('system.mcp.resources.readFailed')
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
.uri-code {
  font-family: monospace;
  font-size: 12px;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  color: #5A67F5;
}
.result-box {
  margin-top: 12px;
  padding: 10px;
  background: #f5f7fa;
  border-radius: 6px;
  max-height: 320px;
  overflow: auto;
  font-size: 12px;
  font-family: monospace;
  white-space: pre-wrap;
}
</style>