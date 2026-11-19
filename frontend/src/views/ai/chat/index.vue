<template>
  <div class="chat-page">
    <!-- 会话侧栏（桌面端常驻，移动端抽屉） -->
    <transition name="session-slide">
      <div v-show="!isMobile || sessionDrawerOpen" class="session-panel">
        <div class="session-panel-header">
          <span class="session-panel-title">{{ t('system.ai.chat.sessions') }}</span>
          <el-tooltip :content="t('system.ai.chat.newSession')" placement="bottom">
            <button class="icon-btn" @click="newSession">
              <el-icon :size="16"><Plus /></el-icon>
            </button>
          </el-tooltip>
        </div>
        <div class="session-list">
          <div
            v-for="s in sessions"
            :key="s.id"
            class="session-item"
            :class="{ active: s.id === activeSessionId }"
            @click="switchSession(s.id)"
          >
            <div class="session-item-row">
              <el-icon class="session-item-icon" :size="15"><ChatDotRound /></el-icon>
              <span class="session-item-title" :title="s.title">{{ s.title }}</span>
              <span v-if="isSessionStreaming(s.id)" class="session-live-dot" :title="t('system.ai.chat.generatingCount', { n: 1 })"></span>
              <el-icon
                class="session-item-del"
                :size="14"
                @click.stop="confirmDeleteSession(s)"
              ><Delete /></el-icon>
            </div>
            <div class="session-item-sub">
              <span>{{ formatSessionTime(s.updated_at) }}</span>
              <span class="session-item-count">{{ t('system.ai.chat.msgCount', { n: s.message_count }) }}</span>
            </div>
          </div>
          <!-- 未落库的进行中新对话（并行生成时可切回） -->
          <div
            v-for="tk in activeTempKeys"
            :key="tk"
            class="session-item"
            :class="{ active: activeKey === tk }"
            @click="switchToTemp(tk)"
          >
            <div class="session-item-row">
              <el-icon class="session-item-icon" :size="15"><EditPen /></el-icon>
              <span class="session-item-title">{{ t('system.ai.chat.newChat') }}</span>
              <span v-if="isKeyStreaming(tk)" class="session-live-dot"></span>
            </div>
            <div class="session-item-sub">
              <span>{{ t('system.ai.chat.generatingCount', { n: 1 }) }}</span>
            </div>
          </div>
          <div v-if="sessions.length === 0 && activeTempKeys.length === 0" class="session-empty">{{ t('system.ai.chat.noSessions') }}</div>
        </div>
      </div>
    </transition>
    <!-- 移动端抽屉遮罩 -->
    <div
      v-if="isMobile && sessionDrawerOpen"
      class="session-overlay"
      @click="sessionDrawerOpen = false"
    ></div>

    <!-- 主聊天区 -->
    <div class="chat-main">
      <!-- 顶栏（轻量：仅会话名；供应商/模型已收纳进底部 composer） -->
      <div class="chat-header">
        <div class="chat-header-left">
          <button v-if="isMobile" class="icon-btn" @click="sessionDrawerOpen = true">
            <el-icon :size="18"><Menu /></el-icon>
          </button>
          <el-icon v-if="streaming" class="header-live-spin" :size="14"><Loading /></el-icon>
          <span class="chat-title">{{ activeTitle }}</span>
        </div>
        <div class="chat-header-right">
          <button class="icon-btn" :title="t('system.ai.chat.renameSession')" @click="renameActiveSession">
            <el-icon :size="16"><EditPen /></el-icon>
          </button>
        </div>
      </div>

      <!-- 消息区域 -->
      <div ref="messagesRef" class="chat-messages" @scroll="handleScroll">
        <!-- 其它会话生成中胶囊（多会话并行） -->
        <div
          v-if="backgroundGenerating.length > 0"
          class="generating-pill"
          @click="jumpToBackgroundSession"
        >
          <el-icon :size="12" class="pill-spin"><Loading /></el-icon>
          <span>{{ t('system.ai.chat.generatingCount', { n: generatingCount }) }}</span>
          <el-icon :size="12"><ArrowRight /></el-icon>
        </div>

        <!-- 欢迎提示 -->
        <div v-if="messages.length === 0" class="chat-welcome">
          <div class="welcome-logo">
            <el-icon :size="30" color="#fff"><ChatDotRound /></el-icon>
          </div>
          <h3>{{ t('system.ai.chat.welcome') }}</h3>
          <p class="welcome-subtitle">{{ t('system.ai.chat.subtitle') }}</p>
          <div class="welcome-suggestions">
            <div
              v-for="s in suggestions"
              :key="s.text"
              class="suggestion-item"
              @click="sendQuickMessage(s.text)"
            >
              <el-icon class="suggestion-icon"><component :is="s.icon" /></el-icon>
              <div class="suggestion-text">
                <div class="suggestion-title">{{ s.title }}</div>
                <div class="suggestion-desc">{{ s.text }}</div>
              </div>
            </div>
          </div>
        </div>

        <!-- 消息列表 -->
        <div
          v-for="(msg, i) in messages"
          :key="i"
          class="message-row"
          :class="msg.role"
        >
          <div class="message-avatar">
            <el-icon v-if="msg.role === 'user'" :size="18" color="#fff"><User /></el-icon>
            <el-icon v-else :size="18" color="#fff"><ChatDotRound /></el-icon>
          </div>
          <div class="message-body">
            <div class="message-meta">
              <span class="message-role">{{ msg.role === 'user' ? t('system.ai.chat.roleUser') : t('system.ai.chat.roleAssistant') }}</span>
            </div>
            <!-- 工具调用状态 -->
            <div v-if="msg.toolEvents && msg.toolEvents.length" class="tool-events">
              <div v-for="(te, j) in msg.toolEvents" :key="j" class="tool-event">
                <el-icon :size="13" :color="te.type === 'tool_result' ? 'var(--el-color-success)' : 'var(--el-color-primary)'">
                  <Loading v-if="te.type === 'tool_call'" class="is-loading" />
                  <CircleCheck v-else />
                </el-icon>
                <span class="tool-name">{{ te.name }}</span>
                <span class="tool-args">{{ formatToolArgs(te.arguments) }}</span>
              </div>
            </div>
            <!-- 消息内容 -->
            <div
              v-if="msg.content"
              class="message-content markdown-body"
              v-html="renderMarkdown(msg.content)"
            ></div>
            <!-- 错误信息 -->
            <div v-if="msg.error" class="message-error">
              <el-icon><WarningFilled /></el-icon>
              {{ msg.error }}
            </div>
            <!-- 正在输入指示 -->
            <div v-if="msg.streaming && !msg.content" class="typing-indicator">
              <span></span><span></span><span></span>
            </div>
            <!-- 消息操作（复制） -->
            <div v-if="msg.content && !msg.streaming" class="message-actions" :class="{ user: msg.role === 'user' }">
              <el-tooltip :content="t('system.ai.chat.copy')" placement="top">
                <button class="msg-action-btn" @click="copyMessage(msg)">
                  <el-icon :size="13"><CopyDocument /></el-icon>
                </button>
              </el-tooltip>
            </div>
          </div>
        </div>
      </div>

      <!-- 输入区域 -->
      <div class="chat-input-area">
        <div class="chat-input-wrapper">
          <el-input
            v-model="inputText"
            type="textarea"
            :rows="2"
            :autosize="{ minRows: 2, maxRows: 6 }"
            :placeholder="t('system.ai.chat.inputPlaceholder')"
            resize="none"
            @keydown.enter.exact.prevent="sendMessage"
          />
          <div class="input-actions">
            <div class="input-actions-left">
              <el-tooltip :content="t('system.ai.chat.toolsTip')" placement="top">
                <div class="tools-toggle">
                  <el-icon :size="14"><SetUp /></el-icon>
                  <span>{{ t('system.ai.chat.tools') }}</span>
                  <el-switch v-model="enableTools" size="small" />
                </div>
              </el-tooltip>
              <el-tooltip :content="t('system.ai.chat.clearChat')" placement="top">
                <button class="composer-icon-btn" @click="clearChat">
                  <el-icon :size="15"><Delete /></el-icon>
                </button>
              </el-tooltip>
            </div>
            <div class="input-actions-right">
              <!-- 供应商 / 模型：无边框透明下拉（收纳进 composer） -->
              <el-select
                v-model="selectedProviderId"
                :placeholder="t('system.ai.chat.provider')"
                class="ghost-select provider-ghost"
                size="small"
                @change="onProviderChange"
              >
                <el-option
                  v-for="p in providers"
                  :key="p.id"
                  :label="p.name"
                  :value="p.id"
                />
              </el-select>
              <el-select
                v-model="selectedModel"
                :placeholder="t('system.ai.chat.model')"
                class="ghost-select model-ghost"
                size="small"
                :disabled="modelOptions.length === 0"
                @change="saveModelChoice"
              >
                <el-option v-for="m in modelOptions" :key="m" :label="m" :value="m" />
              </el-select>
              <button
                v-if="!streaming"
                class="send-btn"
                :disabled="!inputText.trim()"
                :title="t('system.ai.chat.send')"
                @click="sendMessage"
              >
                <el-icon :size="17"><Promotion /></el-icon>
              </button>
              <button v-else class="send-btn stop" :title="t('system.ai.chat.stop')" @click="stopStreaming">
                <el-icon :size="15"><VideoPause /></el-icon>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
// ─────────────────────────────────────────────────────────────────────────
// 模块级会话流状态（跨组件实例保留）
// 放在 setup 外部：切出「AI 全能助手」页面时组件销毁，但后台 run 订阅与
// 视图状态继续存在，切回页面从同一 Map 恢复——「多会话并行、切页面不中断」。
// key 规则：已落库会话 's{id}'；未落库新对话 't{n}'。
// ─────────────────────────────────────────────────────────────────────────
interface ToolEvent {
  type: 'tool_call' | 'tool_result'
  name: string
  arguments?: any
  result?: any
}

interface Message {
  role: 'user' | 'assistant'
  content: string
  streaming?: boolean
  error?: string
  toolEvents?: ToolEvent[]
}

type RunStatus = 'pending' | 'running' | 'done' | 'failed' | 'stopped'

/** 一条后台 run 的前端视图（SSE 订阅持有者） */
interface LiveRun {
  runId: number
  sessionId: number | null
  status: RunStatus
  question: string
  answer: string
  toolEvents: ToolEvent[]
  error: string | null
  asstIdx: number // 对应 st.msgs 中 assistant 占位的下标
  detach: (() => void) | null
  reconnect: number
}

/** 单个会话/新对话的视图状态 */
interface StreamState {
  msgs: Message[]
  run: LiveRun | null
  queue: number[]
  pendingRuns: Map<number, LiveRun>
}

const sessionStreams = new Map<string, StreamState>()
let tempSeq = 0
let lastActiveKey = '' // 记住最后活跃会话，切页/刷新回来恢复

function isTerminalStatus(s: RunStatus | undefined): boolean {
  return s === 'done' || s === 'failed' || s === 'stopped'
}
</script>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, nextTick, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  getAllProviders,
  getChatSessions,
  getChatSessionMessages,
  renameChatSession,
  deleteChatSession,
  createChatRun,
  getChatRun,
  stopChatRun,
  getActiveChatRun,
  subscribeRunEvents,
} from '@/api'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import Prism from 'prismjs'
// 按需注册常用语言
import 'prismjs/components/prism-markup.min.js'
import 'prismjs/components/prism-javascript.min.js'
import 'prismjs/components/prism-typescript.min.js'
import 'prismjs/components/prism-json.min.js'
import 'prismjs/components/prism-python.min.js'
import 'prismjs/components/prism-sql.min.js'
import 'prismjs/components/prism-bash.min.js'
import 'prismjs/components/prism-css.min.js'
import 'prismjs/components/prism-yaml.min.js'
import 'prismjs/components/prism-go.min.js'
import 'prismjs/components/prism-java.min.js'
import 'prismjs/components/prism-php.min.js'
import 'prismjs/components/prism-markdown.min.js'
import 'prismjs/components/prism-ini.min.js'

interface ChatSessionItem {
  id: number
  title: string
  message_count: number
  updated_at: string | null
}

const messagesRef = ref<HTMLElement>()
const { t } = useI18n()
const inputText = ref('')
const enableTools = ref(true)
const providers = ref<any[]>([])
const selectedProviderId = ref<number | null>(null)
const selectedModel = ref<string>('')

const sessions = ref<ChatSessionItem[]>([])
const activeSessionId = ref<number | null>(null)
// 当前未落库新对话的临时 key（null = 无临时对话）
const activeTempKey = ref<string | null>(null)

const sessionDrawerOpen = ref(false)
const isMobile = ref(window.innerWidth <= 991)

const STORAGE_KEY = 'apeadmin_ai_chat_pref'

// ─── 会话流状态桥接（模块级 Map ↔ 响应式视图） ───
// Map 本身非响应式：用版本号驱动视图刷新
const streamVersion = ref(0)
function touchStream() {
  streamVersion.value++
}

/** 当前视图使用的流 key */
const activeKey = computed(() => {
  if (activeSessionId.value) return 's' + activeSessionId.value
  return activeTempKey.value || ''
})

/** 当前会话消息（无状态则空数组展示欢迎页） */
const messages = computed(() => {
  void streamVersion.value // 依赖版本号触发刷新
  return sessionStreams.get(activeKey.value)?.msgs ?? []
})

/** 当前会话是否生成中 */
const streaming = computed(() => {
  void streamVersion.value
  const run = sessionStreams.get(activeKey.value)?.run
  return !!run && !isTerminalStatus(run.status)
})

/** 生成中的会话总数 */
const generatingCount = computed(() => {
  void streamVersion.value
  let n = 0
  sessionStreams.forEach((st) => {
    if (st.run && !isTerminalStatus(st.run.status)) n++
  })
  return n
})

/** 非当前视图的生成中 key（胶囊点击目标） */
const backgroundGenerating = computed(() => {
  void streamVersion.value
  const keys: string[] = []
  sessionStreams.forEach((st, k) => {
    if (st.run && !isTerminalStatus(st.run.status) && k !== activeKey.value) keys.push(k)
  })
  return keys
})

/** 侧栏里展示的、非当前视图的新对话（未落库/生成中） */
const activeTempKeys = computed(() => {
  void streamVersion.value
  const keys: string[] = []
  sessionStreams.forEach((st, k) => {
    if (k.startsWith('t') && k !== activeKey.value && st.msgs.length > 0) keys.push(k)
  })
  return keys
})

/** 顶栏标题：当前会话名 */
const activeTitle = computed(() => {
  if (activeSessionId.value) {
    const s = sessions.value.find((x) => x.id === activeSessionId.value)
    if (s) return s.title
    return t('system.ai.chat.title')
  }
  const st = activeTempKey.value ? sessionStreams.get(activeTempKey.value) : null
  if (st && st.msgs.length) return t('system.ai.chat.newChat')
  return t('system.ai.chat.title')
})

function isSessionStreaming(id: number): boolean {
  void streamVersion.value
  const run = sessionStreams.get('s' + id)?.run
  return !!run && !isTerminalStatus(run.status)
}
function isKeyStreaming(key: string): boolean {
  void streamVersion.value
  const run = sessionStreams.get(key)?.run
  return !!run && !isTerminalStatus(run.status)
}

const currentProvider = computed(() =>
  providers.value.find((p) => p.id === selectedProviderId.value)
)

const modelOptions = computed(() => {
  const p = currentProvider.value
  if (!p) return []
  let models: string[] = []
  try {
    models = typeof p.models === 'string' ? JSON.parse(p.models || '[]') : p.models || []
  } catch {
    models = []
  }
  return models
})

const suggestions = [
  { icon: 'User', title: t('system.ai.chat.suggestQueryUser'), text: t('system.ai.chat.suggestQueryUserText') },
  { icon: 'Avatar', title: t('system.ai.chat.suggestRoleManage'), text: t('system.ai.chat.suggestRoleManageText') },
  { icon: 'DataAnalysis', title: t('system.ai.chat.suggestDataStat'), text: t('system.ai.chat.suggestDataStatText') },
  { icon: 'Menu', title: t('system.ai.chat.suggestMenuTree'), text: t('system.ai.chat.suggestMenuTreeText') },
]

// ---------- 偏好持久化（供应商/模型） ----------
function loadPrefs() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const pref = JSON.parse(raw)
      selectedModel.value = pref.model || ''
      return pref.provider_id || null
    }
  } catch {
    /* ignore */
  }
  return null
}

function saveModelChoice() {
  try {
    localStorage.setItem(
      STORAGE_KEY,
      JSON.stringify({ provider_id: selectedProviderId.value, model: selectedModel.value })
    )
  } catch {
    /* ignore */
  }
}

// ---------- Markdown + 代码高亮 + XSS 防护 ----------
marked.setOptions({
  breaks: true,
  gfm: true,
})

function renderMarkdown(text: string): string {
  try {
    const html = marked.parse(text) as string
    // DOMPurify 清洗，防 XSS；保留 code class 供 Prism 高亮
    const clean = DOMPurify.sanitize(html, {
      ADD_ATTR: ['target'],
    })
    return clean
  } catch {
    return DOMPurify.sanitize(text)
  }
}

/** 渲染完成后对消息区做语法高亮 */
function highlightCode() {
  nextTick(() => {
    if (!messagesRef.value) return
    messagesRef.value.querySelectorAll('pre code').forEach((el) => {
      if (!el.classList.contains('language-marked')) {
        el.classList.add('language-marked')
        try {
          Prism.highlightElement(el as HTMLElement)
        } catch {
          /* 高亮失败忽略，保持原文本 */
        }
      }
    })
  })
}

watch(() => messages.value.map((m) => m.content).join(''), highlightCode)

function formatToolArgs(args: any): string {
  if (!args || typeof args !== 'object') return ''
  const entries = Object.entries(args)
  if (entries.length === 0) return ''
  return entries.map(([k, v]) => `${k}=${v}`).join(', ')
}

// ---------- 供应商与模型 ----------
async function loadProviders() {
  try {
    const data: any = await getAllProviders()
    providers.value = data || []
    const savedProviderId = loadPrefs()
    if (providers.value.length > 0) {
      const saved = providers.value.find((p) => p.id === savedProviderId)
      selectedProviderId.value = saved ? saved.id : providers.value[0].id
      // 模型默认选中：已保存的 > 供应商列表第一个
      if (!modelOptions.value.includes(selectedModel.value)) {
        selectedModel.value = modelOptions.value[0] || ''
      }
    }
  } catch {
    providers.value = []
  }
}

function onProviderChange() {
  // 切换供应商后重置模型为该供应商第一个
  selectedModel.value = modelOptions.value[0] || ''
  saveModelChoice()
}

// ---------- 会话列表 ----------
async function loadSessions() {
  try {
    const data: any = await getChatSessions()
    sessions.value = data?.items || []
  } catch {
    sessions.value = []
  }
}

function upsertSessionItem(item: ChatSessionItem) {
  const idx = sessions.value.findIndex((s) => s.id === item.id)
  if (idx >= 0) sessions.value.splice(idx, 1, { ...sessions.value[idx], ...item })
  else sessions.value.unshift(item)
}

// ---------- 视图状态操作 ----------
function ensureState(key: string): StreamState {
  let st = sessionStreams.get(key)
  if (!st) {
    st = { msgs: [], run: null, queue: [], pendingRuns: new Map() }
    sessionStreams.set(key, st)
  }
  return st
}

function isActiveState(st: StreamState): boolean {
  return sessionStreams.get(activeKey.value) === st
}

function newSession() {
  // 当前已是空白新对话则复用
  const cur = activeTempKey.value ? sessionStreams.get(activeTempKey.value) : null
  if (!activeSessionId.value && cur && cur.msgs.length === 0 && !cur.run) {
    sessionDrawerOpen.value = false
    return
  }
  const key = 't' + ++tempSeq
  sessionStreams.set(key, { msgs: [], run: null, queue: [], pendingRuns: new Map() })
  activeSessionId.value = null
  activeTempKey.value = key
  lastActiveKey = key
  sessionDrawerOpen.value = false
  touchStream()
}

/** 切到某个新对话视图（并行生成时可切回） */
function switchToTemp(key: string) {
  activeSessionId.value = null
  activeTempKey.value = key
  lastActiveKey = key
  sessionDrawerOpen.value = false
  touchStream()
  scrollToBottom(true)
}

/** 跳到最近一个后台生成中的会话 */
function jumpToBackgroundSession() {
  const key = backgroundGenerating.value[0]
  if (!key) return
  if (key.startsWith('s')) {
    activeSessionId.value = Number(key.slice(1))
    activeTempKey.value = null
    lastActiveKey = key
    touchStream()
    scrollToBottom(true)
  } else {
    switchToTemp(key)
  }
}

async function switchSession(id: number) {
  if (activeSessionId.value === id && !activeTempKey.value) return
  activeSessionId.value = id
  activeTempKey.value = null
  const key = 's' + id
  lastActiveKey = key
  sessionDrawerOpen.value = false

  const st = sessionStreams.get(key)
  // 有活跃 run：直接看内存中的实时状态（切页返回场景）
  if (st?.run && !isTerminalStatus(st.run.status)) {
    touchStream()
    scrollToBottom(true)
    return
  }
  try {
    const data: any = await getChatSessionMessages(id)
    if (activeSessionId.value !== id) return // 期间又切走了
    const list = data?.messages || []
    const state = ensureState(key)
    state.msgs = list.map((m: any) => ({
      role: m.role,
      content: m.content,
      toolEvents: m.tool_events || [],
    }))
    state.run = null
    state.queue = []
    state.pendingRuns.clear()
    touchStream()
    scrollToBottom(true)
    // 恢复后台仍在进行的 run（刷新浏览器/其他页面发起）
    await restoreSession(id)
  } catch {
    ElMessage.error(t('system.ai.chat.loadFailed'))
  }
}

/** 进入会话时恢复运行中的 run（刷新页面/重开浏览器后也能续上） */
async function restoreSession(id: number) {
  const key = 's' + id
  const st = sessionStreams.get(key)
  if (!st) return false
  if (st.run && !isTerminalStatus(st.run.status)) return true
  try {
    const data: any = await getActiveChatRun(id)
    if (!data || !data.run_id) return false
    // 历史里已有本轮提问（入队即落库），补一个 assistant 占位承接流式内容
    st.msgs.push({
      role: 'assistant',
      content: data.answer || '',
      streaming: true,
      toolEvents: data.tool_events || [],
    })
    const run: LiveRun = {
      runId: data.run_id,
      sessionId: id,
      status: data.status,
      question: data.question || '',
      answer: data.answer || '',
      toolEvents: data.tool_events || [],
      error: data.error || null,
      asstIdx: st.msgs.length - 1,
      detach: null,
      reconnect: 0,
    }
    // 排队的 run id 跨刷新不可知（后端执行后自然出现），此处清空
    if (data.queued > 0) st.queue = []
    st.run = run
    attach(st, run)
    touchStream()
    return true
  } catch {
    return false
  }
}

async function confirmDeleteSession(s: ChatSessionItem) {
  try {
    await ElMessageBox.confirm(t('system.ai.chat.deleteSession', { title: s.title }), t('common.feedback.tip'), {
      confirmButtonText: t('common.actions.delete'),
      cancelButtonText: t('common.actions.cancel'),
      type: 'warning',
    })
    const key = 's' + s.id
    const st = sessionStreams.get(key)
    // 先停掉该会话的活跃/排队 run，再删库
    if (st) {
      if (st.run && !isTerminalStatus(st.run.status)) {
        try {
          await stopChatRun(st.run.runId)
        } catch {
          /* ignore */
        }
      }
      st.run?.detach?.()
      for (const qid of st.queue) {
        try {
          await stopChatRun(qid)
        } catch {
          /* ignore */
        }
      }
      sessionStreams.delete(key)
    }
    await deleteChatSession(s.id)
    sessions.value = sessions.value.filter((x) => x.id !== s.id)
    if (activeSessionId.value === s.id) {
      activeSessionId.value = null
      activeTempKey.value = null
      lastActiveKey = ''
      newSession()
    }
    touchStream()
    ElMessage.success(t('system.ai.chat.sessionDeleted'))
  } catch {
    /* 用户取消 */
  }
}

async function renameActiveSession() {
  const sid = activeSessionId.value
  if (!sid) {
    ElMessage.info(t('system.ai.chat.renamePrompt'))
    return
  }
  const cur = sessions.value.find((x) => x.id === sid)
  try {
    const res = await ElMessageBox.prompt(
      t('system.ai.chat.renamePrompt'),
      t('system.ai.chat.renameSession'),
      {
        confirmButtonText: t('common.actions.confirm'),
        cancelButtonText: t('common.actions.cancel'),
        inputValue: cur?.title || '',
        inputValidator: (v: string) => (!!v && !!v.trim()) || t('system.ai.chat.renamePrompt'),
      }
    )
    const title = (res.value || '').trim()
    if (!title) return
    await renameChatSession(sid, title)
    if (cur) cur.title = title
    touchStream()
    ElMessage.success(t('system.ai.chat.renameOk'))
  } catch {
    /* 用户取消 */
  }
}

// ---------- 滚动 ----------
let autoScroll = true
function scrollToBottom(force = false) {
  nextTick(() => {
    if (!messagesRef.value) return
    if (force || autoScroll) {
      messagesRef.value.scrollTop = messagesRef.value.scrollHeight
    }
  })
}

function handleScroll() {
  const el = messagesRef.value
  if (!el) return
  // 距底部 80px 内视为跟随滚动
  autoScroll = el.scrollHeight - el.scrollTop - el.clientHeight < 80
}

// ---------- run 订阅 ----------
/** 建立/重建 SSE 订阅（seen=0 时以服务端快照为基线，不会重复） */
function attach(st: StreamState, run: LiveRun, seen = 0) {
  run.detach?.()
  run.detach = subscribeRunEvents(
    run.runId,
    seen,
    (ev: any) => {
      run.reconnect = 0
      if (ev.type === 'snapshot') {
        run.status = ev.status
        run.answer = ev.answer || ''
        run.toolEvents = ev.tool_events || []
        run.error = ev.error || null
      } else if (ev.type === 'run_start') {
        run.status = 'running'
      } else if (ev.type === 'content') {
        run.answer += ev.content || ''
      } else if (ev.type === 'tool_call') {
        run.toolEvents.push({ type: 'tool_call', name: ev.name, arguments: ev.arguments })
      } else if (ev.type === 'tool_result') {
        run.toolEvents.push({ type: 'tool_result', name: ev.name, result: ev.result })
      } else if (ev.type === 'run_end') {
        run.status = ev.status
        run.error = ev.error || null
      } else if (ev.type === 'error') {
        run.status = 'failed'
        run.error = ev.message || t('system.ai.chat.requestFailed')
      }
      syncRunMessage(st, run)
      touchStream()
      if (isActiveState(st)) scrollToBottom()
    },
    () => onRunClosed(st, run),
    (msg: string) => onRunError(st, run, msg)
  )
  syncRunMessage(st, run)
}

/** 把 run 状态同步到消息列表中的 assistant 占位 */
function syncRunMessage(st: StreamState, run: LiveRun) {
  const msg = st.msgs[run.asstIdx]
  if (!msg) return
  msg.content = run.answer
  msg.toolEvents = run.toolEvents
  msg.streaming = !isTerminalStatus(run.status)
  msg.error = run.error || undefined
}

function onRunClosed(st: StreamState, run: LiveRun) {
  if (isTerminalStatus(run.status)) {
    finishRun(st, run)
    return
  }
  if (run.reconnect < 3) {
    reconnectRun(st, run)
    return
  }
  // 多次重连失败：降级为一次性查询
  getChatRun(run.runId)
    .then((d: any) => {
      run.status = d.status
      run.answer = d.answer ?? ''
      run.toolEvents = d.tool_events ?? []
      run.error = d.error || run.error
    })
    .catch(() => {
      run.status = 'failed'
      run.error = run.error || t('system.ai.chat.requestFailed')
    })
    .finally(() => {
      syncRunMessage(st, run)
      touchStream()
      finishRun(st, run)
    })
}

function reconnectRun(st: StreamState, run: LiveRun) {
  run.reconnect += 1
  const delay = Math.min(1000 * 2 ** (run.reconnect - 1), 4000)
  setTimeout(() => {
    if (!isTerminalStatus(run.status)) attach(st, run, 0)
  }, delay)
}

function onRunError(st: StreamState, run: LiveRun, message: string) {
  if (run.reconnect < 3) {
    reconnectRun(st, run)
    return
  }
  run.status = 'failed'
  run.error = run.error || message
  syncRunMessage(st, run)
  touchStream()
  finishRun(st, run)
}

/** run 终态：收尾 + 接管同会话排队中的下一条 */
function finishRun(st: StreamState, run: LiveRun) {
  syncRunMessage(st, run)
  run.detach?.()
  run.detach = null
  touchStream()
  loadSessions()
  while (st.queue.length > 0) {
    const nextId = st.queue.shift()!
    const next = st.pendingRuns.get(nextId)
    st.pendingRuns.delete(nextId)
    if (next && !isTerminalStatus(next.status)) {
      st.run = next
      attach(st, next)
      touchStream()
      return
    }
  }
  st.run = null
  touchStream()
}

// ---------- 发送 ----------
function sendQuickMessage(text: string) {
  inputText.value = text
  sendMessage()
}

async function sendMessage() {
  const text = inputText.value.trim()
  if (!text) return
  if (providers.value.length === 0) {
    ElMessage.warning(t('system.ai.chat.providerRequired'))
    return
  }
  // 确保当前视图有状态容器
  if (!activeKey.value) {
    const tk = 't' + ++tempSeq
    sessionStreams.set(tk, { msgs: [], run: null, queue: [], pendingRuns: new Map() })
    activeTempKey.value = tk
    lastActiveKey = tk
  }
  const key = activeKey.value
  const st = ensureState(key)
  const sessionIdAtSend = activeSessionId.value

  st.msgs.push({ role: 'user', content: text })
  st.msgs.push({ role: 'assistant', content: '', streaming: true, toolEvents: [] })
  const asstIdx = st.msgs.length - 1
  inputText.value = ''
  autoScroll = true
  touchStream()
  scrollToBottom(true)

  try {
    const data: any = await createChatRun({
      session_id: sessionIdAtSend,
      question: text,
      provider_id: selectedProviderId.value,
      model: selectedModel.value || null,
      enable_tools: enableTools.value,
    })
    const runId: number = data.run_id
    const sessionId: number = data.session_id

    // 新对话已落库：视图 key 迁移 t{n} → s{id}，侧栏即时展示
    if (sessionIdAtSend === null) {
      const newKey = 's' + sessionId
      sessionStreams.delete(key)
      sessionStreams.set(newKey, st)
      if (activeTempKey.value === key) {
        activeTempKey.value = null
        activeSessionId.value = sessionId
      }
      if (lastActiveKey === key) lastActiveKey = newKey
      upsertSessionItem({
        id: sessionId,
        title: data.title || t('system.ai.chat.newChatTitle'),
        message_count: 0,
        updated_at: new Date().toISOString(),
      })
    }

    const run: LiveRun = {
      runId,
      sessionId,
      status: 'pending',
      question: text,
      answer: '',
      toolEvents: [],
      error: null,
      asstIdx,
      detach: null,
      reconnect: 0,
    }
    if (st.run && !isTerminalStatus(st.run.status)) {
      // 同会话已有活跃 run：后端 FIFO 排队，当前 run 结束后自动接管
      st.pendingRuns.set(runId, run)
      st.queue.push(runId)
    } else {
      st.run = run
      attach(st, run)
    }
    touchStream()
    scrollToBottom()
  } catch (e: any) {
    const msg = st.msgs[asstIdx]
    if (msg) {
      msg.streaming = false
      msg.error = e?.message || t('system.ai.chat.requestFailed')
    }
    touchStream()
    ElMessage.error(e?.message || t('system.ai.chat.requestFailed'))
  }
}

async function stopStreaming() {
  const st = sessionStreams.get(activeKey.value)
  const run = st?.run
  if (!st || !run || isTerminalStatus(run.status)) return
  try {
    await stopChatRun(run.runId)
  } catch {
    /* 忽略：仍以服务端事件为准 */
  }
  // 无订阅（断网等）时本地兜底收尾
  if (!run.detach) {
    run.status = 'stopped'
    syncRunMessage(st, run)
    touchStream()
    finishRun(st, run)
  }
}

function clearChat() {
  if (messages.value.length === 0) return
  ElMessageBox.confirm(t('system.ai.chat.clearConfirm'), t('common.feedback.tip'), {
    confirmButtonText: t('system.ai.chat.clear'),
    cancelButtonText: t('common.actions.cancel'),
    type: 'warning',
  })
    .then(() => {
      // 仅切换视图（历史仍在会话列表中，可随时切回）
      activeSessionId.value = null
      activeTempKey.value = null
      lastActiveKey = ''
      newSession()
    })
    .catch(() => {})
}

// ---------- 辅助 ----------
function formatSessionTime(iso?: string | null): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  if (d.toDateString() === now.toDateString()) {
    return `${pad(d.getHours())}:${pad(d.getMinutes())}`
  }
  if (d.getFullYear() === now.getFullYear()) {
    return `${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
  }
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

async function copyMessage(msg: Message) {
  try {
    await navigator.clipboard.writeText(msg.content)
    ElMessage.success(t('common.feedback.copySuccess'))
  } catch {
    ElMessage.error(t('common.feedback.copyFailed'))
  }
}

function onResize() {
  isMobile.value = window.innerWidth <= 991
}

onMounted(() => {
  window.addEventListener('resize', onResize)
  loadProviders()
  loadSessions().then(() => {
    // 清理滞留的终态 run（历史中已有最终内容）
    sessionStreams.forEach((st) => {
      if (st.run && isTerminalStatus(st.run.status)) {
        st.run.detach?.()
        st.run = null
      }
    })
    // 恢复上次停留的会话（组件切回/刷新后）
    if (lastActiveKey.startsWith('s')) {
      const id = Number(lastActiveKey.slice(1))
      if (sessions.value.some((s) => s.id === id)) {
        switchSession(id)
      }
    } else if (lastActiveKey.startsWith('t') && sessionStreams.has(lastActiveKey)) {
      activeSessionId.value = null
      activeTempKey.value = lastActiveKey
      touchStream()
    }
  })
})

onBeforeUnmount(() => {
  // 只解绑 DOM 监听；后台 run 订阅挂在模块级 Map 上，切页面不中断
  window.removeEventListener('resize', onResize)
})
</script>

<style scoped>
.chat-page {
  display: flex;
  height: 100%;
  background: var(--theme-body-bg, #f5f7fa);
  border-radius: 8px;
  overflow: hidden;
}

/* ========== 会话侧栏 ========== */
.session-panel {
  width: 232px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--theme-card-bg, #fff);
  border-right: 1px solid var(--border-light, #ebeef5);
}
.session-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border-light, #ebeef5);
  flex-shrink: 0;
}
.session-panel-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #303133);
}
.session-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}
.session-list::-webkit-scrollbar {
  width: 4px;
}
.session-list::-webkit-scrollbar-thumb {
  background: #d0d3d8;
  border-radius: 2px;
}
.session-item {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 2px;
  padding: 8px 10px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--text-regular, #606266);
  font-size: 13px;
  transition: background 0.15s;
}
.session-item-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.session-item-sub {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-left: 23px;
  font-size: 11px;
  line-height: 1.2;
  color: var(--text-secondary, #909399);
}
.session-item-count {
  flex-shrink: 0;
}
.session-live-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #22c55e;
  flex-shrink: 0;
  animation: live-breathe 1.4s ease-in-out infinite;
}
@keyframes live-breathe {
  0%, 100% { opacity: 0.35; transform: scale(0.85); }
  50% { opacity: 1; transform: scale(1); }
}
.session-item:hover {
  background: var(--fill-hover, #f5f7fa);
}
.session-item.active {
  background: var(--el-color-primary-light-9, #ecf0ff);
  color: var(--el-color-primary, #5a67f5);
}
.session-item-icon {
  flex-shrink: 0;
}
.session-item-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.session-item-del {
  flex-shrink: 0;
  color: var(--text-secondary, #909399);
  opacity: 0;
  transition: opacity 0.15s;
}
.session-item:hover .session-item-del {
  opacity: 1;
}
.session-item-del:hover {
  color: var(--el-color-danger, #f56c6c);
}
.session-empty {
  text-align: center;
  color: var(--text-secondary, #909399);
  font-size: 12px;
  padding: 24px 0;
}
.session-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
  z-index: 20;
}

/* ========== 主区 ========== */
.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

/* Header */
.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background: var(--theme-card-bg, #fff);
  border-bottom: 1px solid var(--border-light, #ebeef5);
  flex-shrink: 0;
  gap: 8px;
}
.chat-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.chat-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary, #303133);
  white-space: nowrap;
}
.chat-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
.header-live-spin {
  color: var(--el-color-primary, #5a67f5);
  animation: spin 1s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-regular, #606266);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.icon-btn:hover {
  background: var(--fill-hover, #f5f7fa);
  color: var(--el-color-primary, #5a67f5);
}

/* Messages */
.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  scroll-behavior: smooth;
}
.chat-messages::-webkit-scrollbar {
  width: 6px;
}
.chat-messages::-webkit-scrollbar-thumb {
  background: #d0d3d8;
  border-radius: 3px;
}

/* 生成中胶囊（多会话并行时提示后台生成） */
.generating-pill {
  position: sticky;
  top: 0;
  z-index: 5;
  display: flex;
  align-items: center;
  gap: 6px;
  width: fit-content;
  margin: 0 auto 12px;
  padding: 5px 12px;
  border-radius: 999px;
  background: var(--el-color-primary-light-9, #ecf0ff);
  color: var(--el-color-primary, #5a67f5);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  box-shadow: 0 2px 8px rgba(90, 103, 245, 0.12);
  transition: transform 0.15s, box-shadow 0.15s;
}
.generating-pill:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(90, 103, 245, 0.2);
}
.pill-spin {
  animation: spin 1s linear infinite;
}

/* Welcome */
.chat-welcome {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  text-align: center;
  padding: 24px;
}
.welcome-logo {
  width: 56px;
  height: 56px;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--el-color-primary, #5a67f5), #47d8ff);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 8px 20px rgba(90, 103, 245, 0.25);
}
.chat-welcome h3 {
  margin: 16px 0 6px;
  font-size: 19px;
  color: var(--text-primary, #303133);
}
.welcome-subtitle {
  margin: 0 0 22px;
  font-size: 13px;
  color: var(--text-secondary, #909399);
}
.welcome-suggestions {
  display: grid;
  grid-template-columns: repeat(2, minmax(200px, 260px));
  gap: 10px;
}
.suggestion-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  background: var(--theme-card-bg, #fff);
  border: 1px solid var(--border-light, #ebeef5);
  border-radius: 10px;
  cursor: pointer;
  text-align: left;
  transition: all 0.2s;
}
.suggestion-item:hover {
  border-color: var(--el-color-primary, #5a67f5);
  color: var(--el-color-primary, #5a67f5);
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(90, 103, 245, 0.1);
}
.suggestion-icon {
  margin-top: 2px;
  color: var(--el-color-primary, #5a67f5);
}
.suggestion-text {
  min-width: 0;
}
.suggestion-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #303133);
}
.suggestion-item:hover .suggestion-title {
  color: var(--el-color-primary, #5a67f5);
}
.suggestion-desc {
  font-size: 12px;
  color: var(--text-secondary, #909399);
  margin-top: 2px;
}

/* Message row */
.message-row {
  display: flex;
  gap: 10px;
  margin-bottom: 18px;
  max-width: 860px;
  margin-left: auto;
  margin-right: auto;
}
.message-row.user {
  flex-direction: row-reverse;
}
.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.message-row.user .message-avatar {
  background: var(--el-color-primary, #5a67f5);
}
.message-row.assistant .message-avatar {
  background: linear-gradient(135deg, var(--el-color-primary, #5a67f5), #47d8ff);
}
.message-body {
  flex: 1;
  min-width: 0;
}
.message-meta {
  margin-bottom: 4px;
}
.message-role {
  font-size: 12px;
  color: var(--text-secondary, #909399);
}
.message-row.user .message-meta {
  text-align: right;
}

/* Message content */
.message-content {
  display: inline-block;
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.65;
  text-align: left;
  word-break: break-word;
  max-width: 100%;
}
.message-row.user .message-content {
  background: var(--el-color-primary, #5a67f5);
  color: #fff;
  border-top-right-radius: 4px;
}
.message-row.assistant .message-content {
  background: var(--theme-card-bg, #fff);
  color: var(--text-primary, #303133);
  border: 1px solid var(--border-light, #ebeef5);
  border-top-left-radius: 4px;
}

/* 消息操作 */
.message-actions {
  display: flex;
  gap: 4px;
  margin-top: 4px;
  opacity: 0;
  transition: opacity 0.15s;
}
.message-row:hover .message-actions {
  opacity: 1;
}
.message-actions.user {
  justify-content: flex-end;
}
.msg-action-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--text-secondary, #909399);
  cursor: pointer;
  transition: all 0.15s;
}
.msg-action-btn:hover {
  background: var(--fill-hover, #f5f7fa);
  color: var(--el-color-primary, #5a67f5);
}

/* Markdown styling inside assistant messages */
.message-row.assistant .markdown-body :deep(h1),
.message-row.assistant .markdown-body :deep(h2),
.message-row.assistant .markdown-body :deep(h3) {
  margin: 8px 0 4px;
  font-weight: 600;
}
.message-row.assistant .markdown-body :deep(p) {
  margin: 4px 0;
}
.message-row.assistant .markdown-body :deep(ul),
.message-row.assistant .markdown-body :deep(ol) {
  padding-left: 20px;
  margin: 4px 0;
}
.message-row.assistant .markdown-body :deep(code) {
  background: var(--fill-light, #f0f2f5);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
  font-family: 'SF Mono', 'Consolas', monospace;
}
.message-row.assistant .markdown-body :deep(pre) {
  background: #1e1e2e;
  color: #d4d4d4;
  padding: 12px 14px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 8px 0;
  font-size: 13px;
}
.message-row.assistant .markdown-body :deep(pre code) {
  background: transparent;
  padding: 0;
  color: inherit;
  font-family: 'SF Mono', 'Consolas', monospace;
}
.message-row.assistant .markdown-body :deep(table) {
  border-collapse: collapse;
  margin: 8px 0;
  width: 100%;
}
.message-row.assistant .markdown-body :deep(th),
.message-row.assistant .markdown-body :deep(td) {
  border: 1px solid var(--border-light, #ebeef5);
  padding: 6px 12px;
  text-align: left;
}
.message-row.assistant .markdown-body :deep(th) {
  background: var(--fill-light, #f5f7fa);
  font-weight: 600;
}
/* Prism token 颜色（自定义克制配色） */
.message-row.assistant .markdown-body :deep(.token.comment),
.message-row.assistant .markdown-body :deep(.token.prolog),
.message-row.assistant .markdown-body :deep(.token.doctype),
.message-row.assistant .markdown-body :deep(.token.cdata) {
  color: #6c7293;
}
.message-row.assistant .markdown-body :deep(.token.punctuation) {
  color: #9aa0b5;
}
.message-row.assistant .markdown-body :deep(.token.keyword),
.message-row.assistant .markdown-body :deep(.token.tag),
.message-row.assistant .markdown-body :deep(.token.selector) {
  color: #c792ea;
}
.message-row.assistant .markdown-body :deep(.token.string),
.message-row.assistant .markdown-body :deep(.token.attr-value),
.message-row.assistant .markdown-body :deep(.token.char) {
  color: #c3e88d;
}
.message-row.assistant .markdown-body :deep(.token.number),
.message-row.assistant .markdown-body :deep(.token.boolean) {
  color: #f78c6c;
}
.message-row.assistant .markdown-body :deep(.token.function),
.message-row.assistant .markdown-body :deep(.token.class-name) {
  color: #82aaff;
}
.message-row.assistant .markdown-body :deep(.token.attr-name),
.message-row.assistant .markdown-body :deep(.token.property) {
  color: #ffcb6b;
}

/* Tool events */
.tool-events {
  margin-bottom: 6px;
}
.tool-event {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: var(--el-color-primary-light-9, rgba(90, 103, 245, 0.06));
  border-radius: 6px;
  font-size: 12px;
  color: var(--text-regular, #606266);
  margin-bottom: 2px;
}
.tool-name {
  font-weight: 600;
  color: var(--el-color-primary, #5a67f5);
}
.tool-args {
  color: var(--text-secondary, #909399);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* Error */
.message-error {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: var(--el-color-danger-light-9, #fef0f0);
  border-radius: 8px;
  color: var(--el-color-danger, #f56c6c);
  font-size: 13px;
}

/* Typing indicator */
.typing-indicator {
  display: inline-flex;
  gap: 4px;
  padding: 12px 16px;
  background: var(--theme-card-bg, #fff);
  border: 1px solid var(--border-light, #ebeef5);
  border-radius: 12px;
}
.typing-indicator span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #c0c4cc;
  animation: typing 1.4s infinite ease-in-out;
}
.typing-indicator span:nth-child(2) {
  animation-delay: 0.2s;
}
.typing-indicator span:nth-child(3) {
  animation-delay: 0.4s;
}
@keyframes typing {
  0%, 60%, 100% { transform: translateY(0); opacity: 0.4; }
  30% { transform: translateY(-6px); opacity: 1; }
}

/* Input area */
.chat-input-area {
  padding: 12px 16px 16px;
  background: var(--theme-card-bg, #fff);
  border-top: 1px solid var(--border-light, #ebeef5);
  flex-shrink: 0;
}
.chat-input-wrapper {
  max-width: 860px;
  margin: 0 auto;
}
.input-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 8px;
  gap: 8px;
}
.input-actions-left {
  display: flex;
  align-items: center;
  gap: 2px;
}
.tools-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary, #909399);
  cursor: default;
}
.input-actions-right {
  display: flex;
  align-items: center;
  gap: 6px;
}
/* 透明无边框下拉（供应商/模型收纳进 composer） */
.ghost-select {
  width: auto;
}
.provider-ghost {
  max-width: 150px;
}
.model-ghost {
  max-width: 170px;
}
.ghost-select :deep(.el-select__wrapper) {
  background: transparent;
  box-shadow: none;
  border: none;
  padding: 0 6px;
  min-height: 26px;
}
.ghost-select :deep(.el-select__wrapper:hover) {
  background: var(--fill-hover, #f5f7fa);
}
.ghost-select :deep(.el-select__selected-item) {
  font-size: 12px;
  color: var(--text-secondary, #909399);
}
/* composer 图标按钮（清空） */
.composer-icon-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  margin-left: 6px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text-secondary, #909399);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.composer-icon-btn:hover {
  background: var(--fill-hover, #f5f7fa);
  color: var(--el-color-danger, #f56c6c);
}
/* 圆形发送/停止按钮 */
.send-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  border: none;
  border-radius: 50%;
  background: linear-gradient(135deg, var(--el-color-primary, #5a67f5), #47d8ff);
  color: #fff;
  cursor: pointer;
  transition: transform 0.15s, box-shadow 0.15s, opacity 0.15s;
  box-shadow: 0 2px 8px rgba(90, 103, 245, 0.25);
}
.send-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(90, 103, 245, 0.35);
}
.send-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
  box-shadow: none;
}
.send-btn.stop {
  background: linear-gradient(135deg, #f56c6c, #f89898);
  box-shadow: 0 2px 8px rgba(245, 108, 108, 0.25);
}

/* ========== 移动端适配（≤991px） ========== */
@media (max-width: 991px) {
  .chat-page {
    position: relative;
    border-radius: 0;
  }
  .session-panel {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    z-index: 21;
    box-shadow: 4px 0 16px rgba(0, 0, 0, 0.08);
    transform: translateX(0);
  }
  .provider-ghost {
    max-width: 96px;
  }
  .model-ghost {
    max-width: 112px;
  }
  .chat-messages {
    padding: 14px;
  }
  .message-row {
    gap: 8px;
    margin-bottom: 14px;
  }
  .message-avatar {
    width: 28px;
    height: 28px;
  }
  .message-content {
    font-size: 13.5px;
    padding: 9px 12px;
  }
  .welcome-suggestions {
    grid-template-columns: 1fr;
    width: 100%;
    max-width: 340px;
  }
  .chat-input-area {
    padding: 10px 12px calc(12px + env(safe-area-inset-bottom, 0px));
  }
}

/* 会话抽屉滑入动画 */
.session-slide-enter-active,
.session-slide-leave-active {
  transition: transform 0.25s ease, opacity 0.25s ease;
}
.session-slide-enter-from,
.session-slide-leave-to {
  transform: translateX(-100%);
  opacity: 0;
}
</style>

<style>
/* ===== 深色模式适配 ===== */
html.dark .session-panel {
  background: #232838;
  border-right-color: #36394a;
}
html.dark .session-panel-header {
  border-bottom-color: #36394a;
}
html.dark .session-panel-title {
  color: #e6e8f0;
}
html.dark .session-item {
  color: #b8bdd0;
}
html.dark .session-item:hover {
  background: rgba(127, 138, 248, 0.08);
}
html.dark .session-item.active {
  background: rgba(127, 138, 248, 0.15);
  color: #7f8af8;
}
html.dark .chat-header {
  background: #232838;
  border-bottom-color: #36394a;
}
html.dark .chat-title {
  color: #e6e8f0;
}
html.dark .chat-welcome h3 {
  color: #e6e8f0;
}
html.dark .suggestion-item {
  background: #232838;
  border-color: #36394a;
}
html.dark .suggestion-title {
  color: #e6e8f0;
}
html.dark .suggestion-item:hover {
  border-color: #7f8af8;
  background: rgba(127, 138, 248, 0.06);
}
html.dark .message-row.assistant .message-content {
  background: #232838;
  color: #e6e8f0;
  border-color: #36394a;
}
html.dark .message-row.assistant .markdown-body code {
  background: #2e3344;
}
html.dark .message-row.assistant .markdown-body th {
  background: #2e3344;
}
html.dark .message-row.assistant .markdown-body th,
html.dark .message-row.assistant .markdown-body td {
  border-color: #36394a;
}
html.dark .tool-event {
  background: rgba(127, 138, 248, 0.1);
  color: #b8bdd0;
}
html.dark .message-error {
  background: rgba(245, 108, 108, 0.1);
  color: #f87171;
}
html.dark .typing-indicator {
  background: #232838;
  border-color: #36394a;
}
html.dark .typing-indicator span {
  background: #6b7089;
}
html.dark .chat-input-area {
  background: #232838;
  border-top-color: #36394a;
}
html.dark .chat-input-area .el-textarea__inner {
  background: #2e3344;
  border-color: #3a3f52;
  color: #e6e8f0;
}
html.dark .chat-input-area .el-textarea__inner::placeholder {
  color: #6b7089;
}
html.dark .msg-action-btn:hover {
  background: rgba(127, 138, 248, 0.1);
  color: #7f8af8;
}
html.dark .session-item-sub {
  color: #6b7089;
}
html.dark .generating-pill {
  background: rgba(127, 138, 248, 0.15);
  color: #9aa3ff;
}
html.dark .ghost-select .el-select__wrapper:hover {
  background: rgba(127, 138, 248, 0.1);
}
html.dark .ghost-select .el-select__selected-item {
  color: #9aa0b5;
}
html.dark .composer-icon-btn:hover {
  background: rgba(127, 138, 248, 0.1);
}
</style>
