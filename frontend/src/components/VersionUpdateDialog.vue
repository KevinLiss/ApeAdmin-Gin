<template>
  <el-dialog
    v-model="visible"
    title="版本更新"
    width="520px"
    :close-on-click-modal="false"
    :close-on-press-escape="!uploading"
    :show-close="!uploading"
    append-to-body
    class="version-update-dialog"
  >
    <!-- Current version info -->
    <div v-loading="loading" class="version-info">
      <div class="version-row">
        <span class="version-label">当前版本</span>
        <span class="version-value">{{ versionData?.current_version || '--' }}</span>
      </div>
      <div class="version-row">
        <span class="version-label">应用名称</span>
        <span class="version-value">{{ versionData?.app_name || '--' }}</span>
      </div>
      <div class="version-row">
        <span class="version-label">底座类型</span>
        <span class="version-value">{{ runtimeLabel }}</span>
      </div>
      <div class="version-row">
        <span class="version-label">Python</span>
        <span class="version-value">{{ versionData?.python_version || '--' }}</span>
      </div>
    </div>

    <!-- Upload area -->
    <el-upload
      ref="uploadRef"
      class="version-upload"
      drag
      :auto-upload="false"
      :limit="1"
      :on-change="handleFileChange"
      :on-exceed="handleExceed"
      accept=".tar.gz,.tgz"
      :disabled="uploading"
    >
      <el-icon class="el-icon--upload"><UploadFilled /></el-icon>
      <div class="el-upload__text">
        拖拽部署包到此处，或<em>点击选择</em>
      </div>
      <template #tip>
        <div class="el-upload__tip">
          仅支持 Python 版 .tar.gz 升级包（build_deploy_package.sh 生成），最大 200MB；插件包（含 plugin.json）请到「插件管理 → 导入插件」
        </div>
      </template>
    </el-upload>

    <!-- Progress bar -->
    <div v-if="uploading" class="upload-progress">
      <el-progress :percentage="progress" :status="progressStatus" />
      <p class="progress-text">{{ progressText }}</p>
    </div>

    <!-- Error message -->
    <el-alert
      v-if="errorMsg"
      :title="errorMsg"
      type="error"
      :closable="false"
      show-icon
      class="upload-error"
    />

    <!-- Success message -->
    <el-alert
      v-if="successMsg"
      :title="successMsg"
      type="success"
      :closable="false"
      show-icon
      class="upload-success"
    />

    <!-- Actions -->
    <template #footer>
      <div class="dialog-footer">
        <el-button @click="handleClose" :disabled="uploading">关闭</el-button>
        <el-button
          type="primary"
          :loading="uploading"
          :disabled="!selectedFile || !!successMsg"
          @click="handleUpload"
        >
          {{ uploading ? '上传中...' : '开始更新' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import type { UploadFile, UploadFiles, UploadRawFile } from 'element-plus'
import { getSystemVersion, uploadSystemUpdate } from '@/api'
import { pollBackendHealth } from '@/utils/restart'

const props = defineProps<{
  modelValue: boolean
}>()
const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

const visible = ref(props.modelValue)
watch(() => props.modelValue, (val) => {
  visible.value = val
  if (val) {
    fetchVersion()
  }
})
watch(visible, (val) => {
  emit('update:modelValue', val)
})

// ---- Version info ----
const loading = ref(false)
const versionData = ref<any>(null)

// Base-stack identity label: backend reports runtime="python"|"go".
const runtimeLabel = computed(() => {
  const rt = String(versionData.value?.runtime || 'python').toLowerCase()
  if (rt === 'go') return 'Go 版（ApeAdmin-Gin）'
  if (rt === 'python') return 'Python 版（FastAPI）'
  return rt || '--'
})

async function fetchVersion() {
  loading.value = true
  try {
    const res: any = await getSystemVersion()
    // axios 拦截器已解包标准信封，res 就是 data 本体（含 current_version 等字段）
    versionData.value = res || null
  } catch (err: any) {
    // silently fail, show -- placeholders
    console.error('Failed to fetch version:', err)
  } finally {
    loading.value = false
  }
}

// ---- Upload ----
const uploadRef = ref()
const selectedFile = ref<File | null>(null)
const uploading = ref(false)
const progress = ref(0)
const progressStatus = ref<any>('')
const progressText = ref('')
const errorMsg = ref('')
const successMsg = ref('')

function handleFileChange(file: UploadFile, files: UploadFiles) {
  // Only keep the last file
  if (files.length > 1) {
    files.splice(0, files.length - 1)
  }
  const raw = file.raw as UploadRawFile
  // Validate extension
  const name = raw.name.toLowerCase()
  if (!name.endsWith('.tar.gz') && !name.endsWith('.tgz')) {
    errorMsg.value = '请选择 .tar.gz 格式的部署包'
    selectedFile.value = null
    uploadRef.value?.clearFiles()
    return
  }
  // Validate size (200MB)
  if (raw.size > 200 * 1024 * 1024) {
    errorMsg.value = '文件大小不能超过 200MB'
    selectedFile.value = null
    uploadRef.value?.clearFiles()
    return
  }
  errorMsg.value = ''
  selectedFile.value = raw
}

function handleExceed(files: File[]) {
  ElMessage.warning('只能选择一个文件，已替换为新文件')
  uploadRef.value?.clearFiles()
  const file = files[0]
  uploadRef.value?.handleStart(file)
}

async function handleUpload() {
  if (!selectedFile.value) return

  uploading.value = true
  progress.value = 0
  progressStatus.value = ''
  progressText.value = '准备上传...'
  errorMsg.value = ''
  successMsg.value = ''

  try {
    const res: any = await uploadSystemUpdate(selectedFile.value, (pct: number) => {
      progress.value = pct
      if (pct < 100) {
        progressText.value = `上传中... ${pct}%`
      } else {
        progressText.value = '上传完成，正在处理...'
        progressStatus.value = 'success'
      }
    })

    progress.value = 100
    progressStatus.value = 'success'
    progressText.value = '更新完成，后端正在重启...'
    // axios 拦截器已解包标准信封：res 直接就是 data（含 old_pid / message）
    successMsg.value = res?.message || '版本更新完成，后端正在重启，请等待约 5 秒后刷新页面'

    // Poll health: wait for the new process to come up (old PID known).
    progressText.value = '等待后端重启完成...'
    const oldPid = res?.old_pid
    const pollResult = await pollBackendHealth({
      oldPid,
      maxRetries: 30,
      interval: 2000,
      onProbe: (isDown, _pid, attempt) => {
        if (isDown) {
          progressText.value = `等待后端重启完成... (${attempt}/30)`
        }
      },
    })
    if (pollResult.recovered) {
      progressText.value = '后端已恢复，正在刷新页面...'
      uploading.value = false
      setTimeout(() => window.location.reload(), 1000)
    } else {
      progressText.value = '后端重启超时，请手动刷新页面'
      uploading.value = false
    }
    } catch (err: any) {
    progressStatus.value = 'exception'
    // Backend returns { code, msg, data } in the standard envelope.
    errorMsg.value =
      err?.response?.data?.msg
      || err?.response?.data?.detail?.msg
      || err?.response?.data?.detail
      || err?.message
      || '上传失败，请重试'
    uploading.value = false
  }
}

function handleClose() {
  if (uploading.value) return
  visible.value = false
  // Reset state
  selectedFile.value = null
  uploading.value = false
  progress.value = 0
  progressStatus.value = ''
  progressText.value = ''
  errorMsg.value = ''
  successMsg.value = ''
  uploadRef.value?.clearFiles()
}

// Reset state when dialog opens
watch(() => props.modelValue, (val) => {
  if (val) {
    errorMsg.value = ''
    successMsg.value = ''
    progress.value = 0
    progressText.value = ''
    uploading.value = false
  }
})
</script>

<style scoped>
.version-update-dialog :deep(.el-dialog__body) {
  padding: 20px 24px;
}

.version-info {
  background: #f8fafc;
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 20px;
}

.version-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 0;
}

.version-row + .version-row {
  border-top: 1px solid #eef0f4;
}

.version-label {
  font-size: 13px;
  color: #909399;
}

.version-value {
  font-size: 14px;
  font-weight: 500;
  color: #2b2b2b;
}

.version-upload {
  width: 100%;
}

.version-upload :deep(.el-upload-dragger) {
  padding: 20px;
  border-radius: 8px;
}

.upload-progress {
  margin-top: 16px;
}

.progress-text {
  margin: 8px 0 0;
  font-size: 12px;
  color: #909399;
  text-align: center;
}

.upload-error,
.upload-success {
  margin-top: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}
</style>

<style>
html.dark .version-info {
  background: #2e3344;
}
html.dark .version-row + .version-row {
  border-top-color: #3a3f52;
}
html.dark .version-label {
  color: #8a90a8;
}
html.dark .version-value {
  color: #e6e8f0;
}
</style>
