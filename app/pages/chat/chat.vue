<template>
  <view class="page" :style="{ height: pageHeight + 'px' }">
    <!-- 会话列表视图 -->
    <view v-if="!currentConv" class="conv-view">
      <view class="nav-bar">
        <text class="nav-back" @click="navigateBack">‹</text>
        <text class="nav-title">{{ wsName }}</text>
        <text class="nav-action" @click="createConv">+ 新对话</text>
      </view>
      <scroll-view scroll-y class="conv-scroll">
        <view v-if="conversations.length === 0" class="empty"><text>暂无对话，点击右上角新建</text></view>
        <view v-else class="list-block">
          <view v-for="c in conversations" :key="c.id" class="list-item" @click="openConv(c)">
            <view class="item-body">
              <text class="item-title">{{ c.title || '新对话' }}</text>
              <text class="item-tokens">{{ c.total_tokens ? c.total_tokens + ' tokens' : '' }}</text>
            </view>
            <text :class="'conv-badge s' + c.status">{{ convStatusText(c.status) }}</text>
            <text class="arrow">›</text>
          </view>
        </view>
      </scroll-view>
    </view>

    <!-- 对话视图 -->
    <view v-else class="chat-view">
      <view class="nav-bar">
        <text class="nav-back" @click="goBack">‹</text>
        <text class="nav-title">{{ currentConv.title || '新对话' }}</text>
        <text class="nav-tasks" @click="openTasks">任务</text>
        <text v-if="isRunning" class="nav-stop" @click="stop">停止</text>
      </view>

      <scroll-view class="messages" scroll-y :scroll-into-view="scrollToId">
        <view v-for="(m, i) in messages" :key="i" :id="'m' + i" :class="['msg-row', m.role]">
          <!-- 状态消息 -->
          <view v-if="m.role === 'status'" class="status-notice">
            <text class="status-text">{{ m.blocks[0].content }}</text>
          </view>
          <!-- 用户/助手消息 -->
          <view v-else :class="['bubble', m.role]">
            <template v-for="(b, bi) in m.blocks" :key="bi">
              <!-- 文本块 -->
              <view v-if="b.type === 'text'" class="block-text">
                <text class="msg-text" :user-select="true">{{ b.content }}</text>
              </view>
              <!-- 思考过程 -->
              <view v-else-if="b.type === 'reasoning'" class="block-reasoning" @click="b.collapsed = !b.collapsed">
                <view class="block-header">
                  <text class="block-icon">💭</text>
                  <text class="block-label reasoning-label">思考过程</text>
                  <text class="block-toggle">{{ b.collapsed ? '›' : '⌄' }}</text>
                </view>
                <text v-if="!b.collapsed" class="reasoning-body" :user-select="true">{{ b.content }}</text>
              </view>
              <!-- 工具调用 -->
              <view v-else-if="b.type === 'tool_call'" class="block-tool-call" @click="b.collapsed = !b.collapsed">
                <view class="block-header">
                  <text class="block-icon">🔧</text>
                  <text class="block-label tool-call-label">{{ b.name }}</text>
                  <text class="block-toggle">{{ b.collapsed ? '›' : '⌄' }}</text>
                </view>
                <view v-if="!b.collapsed" class="tool-call-args">
                  <text class="code-text" :user-select="true">{{ b.args }}</text>
                </view>
              </view>
              <!-- 工具结果 -->
              <view v-else-if="b.type === 'tool_result'" :class="['block-tool-result', { 'is-error': b.isError }]"
                    @click="b.collapsed = !b.collapsed">
                <view class="block-header">
                  <text class="result-icon" :class="b.isError ? 'icon-err' : 'icon-ok'">{{ b.isError ? '✗' : '✓' }}</text>
                  <text class="block-label result-preview">{{ resultPreview(b) }}</text>
                  <text class="block-toggle">{{ b.collapsed ? '›' : '⌄' }}</text>
                </view>
                <view v-if="!b.collapsed" class="tool-result-body">
                  <text class="code-text" :user-select="true">{{ b.content }}</text>
                </view>
              </view>
              <!-- 错误块 -->
              <view v-else-if="b.type === 'error'" class="block-error">
                <text class="error-text">{{ b.content }}</text>
              </view>
              <!-- 代码块 -->
              <view v-else-if="b.type === 'code'" class="block-code">
                <text v-if="b.lang" class="code-lang">{{ b.lang }}</text>
                <text class="code-text" :user-select="true">{{ b.content }}</text>
              </view>
            </template>
          </view>
        </view>

        <!-- 流式消息 -->
        <view v-if="isRunning && streamingBlocks.length" class="msg-row assistant" id="m-streaming">
          <view class="bubble assistant streaming">
            <template v-for="(b, bi) in streamingBlocks" :key="bi">
              <view v-if="b.type === 'text'" class="block-text">
                <text class="msg-text">{{ b.content }}</text>
              </view>
              <view v-else-if="b.type === 'reasoning'" class="block-reasoning">
                <view class="block-header">
                  <text class="block-icon">💭</text>
                  <text class="block-label reasoning-label">思考中...</text>
                </view>
                <text class="reasoning-body streaming-reason">{{ b.content }}</text>
              </view>
              <view v-else-if="b.type === 'tool_call'" class="block-tool-call">
                <view class="block-header">
                  <text class="block-icon">🔧</text>
                  <text class="block-label tool-call-label">{{ b.name }}</text>
                </view>
              </view>
              <view v-else-if="b.type === 'tool_result'" :class="['block-tool-result', { 'is-error': b.isError }]">
                <view class="block-header">
                  <text class="result-icon">{{ b.isError ? '✗' : '✓' }}</text>
                  <text class="block-label">{{ b.isError ? '执行失败' : '执行完成' }}</text>
                </view>
              </view>
              <view v-else-if="b.type === 'error'" class="block-error">
                <text class="error-text">{{ b.content }}</text>
              </view>
            </template>
          </view>
        </view>
        <view v-else-if="isRunning" class="msg-row assistant">
          <view class="bubble assistant streaming">
            <text class="msg-text typing-dot">···</text>
          </view>
        </view>
        <view id="m-end"></view>
      </scroll-view>

      <view class="input-bar">
        <textarea :value="inputText" @input="inputText = $event.detail.value"
          placeholder="输入消息..." class="input" :disabled="isRunning"
          :autoHeight="true" :cursor-spacing="20" :adjust-position="true" @confirm="send" />
        <text class="send-btn" :class="{ active: inputText && !isRunning }" @click="send">发送</text>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref, nextTick } from 'vue'
import { onLoad, onUnload } from '@dcloudio/uni-app'
import { getCurrentServer } from '../../utils/storage.js'
import { listConversations, createConversation, getMessages, stopConversation, sendMessageSSE } from '../../utils/api.js'
import { parseHistoryMessages, formatArgs, truncate } from '../../utils/messageParser.js'

const wsId = ref(null)
const wsName = ref('')
const conversations = ref([])
const currentConv = ref(null)
const messages = ref([])
const streamingBlocks = ref([])
const streamState = ref({ inThinkTag: false, textIdx: -1 })
const isRunning = ref(false)
const inputText = ref('')
const lastSeq = ref(0)
const scrollToId = ref('')
const sseXhr = ref(null)

const sysInfo = uni.getSystemInfoSync()
const pageHeight = sysInfo.windowHeight

onLoad((opts) => {
  wsId.value = parseInt(opts.wsId)
  wsName.value = decodeURIComponent(opts.wsName || '工作区')
  loadConvs()
})

onUnload(() => { if (sseXhr.value) sseXhr.value.abort() })

async function loadConvs() {
  const res = await listConversations(wsId.value).catch(() => ({ data: [] }))
  conversations.value = res.data || []
}

async function createConv() {
  const res = await createConversation(wsId.value, 0).catch(() => null)
  if (!res) return
  conversations.value.unshift(res.data)
  openConv(res.data)
}

async function openConv(conv) {
  currentConv.value = conv
  messages.value = []
  streamingBlocks.value = []
  streamState.value = { inThinkTag: false, textIdx: -1 }
  lastSeq.value = 0
  const res = await getMessages(conv.id, wsId.value).catch(() => null)
  if (res) {
    messages.value = parseHistoryMessages(res.data?.messages || [])
    isRunning.value = res.data?.status === 1
    if (isRunning.value) attachRunning()
  }
  scrollBottom()
}

function goBack() { currentConv.value = null; lastSeq.value = 0 }
function navigateBack() { uni.redirectTo({ url: '/pages/workbench/workbench' }) }
function openTasks() { uni.navigateTo({ url: `/pages/tasks/tasks?wsId=${wsId.value}` }) }

function resultPreview(b) {
  if (b.isError) return truncate(b.content, 40) || '执行失败'
  return truncate(b.content, 50) || '执行完成'
}

function attachRunning() {
  const s = getCurrentServer()
  sseXhr.value = sendMessageSSE({
    baseURL: s.baseURL, token: s.token,
    convId: currentConv.value.id, workspaceId: wsId.value,
    message: '', lastSeq: lastSeq.value,
    onEvent: e => handleEvent(e),
    onDone: () => { isRunning.value = false; flushStreaming() },
    onError: () => { if (isRunning.value) setTimeout(() => attachRunning(), 3000) }
  })
}

function send() {
  if (!inputText.value.trim() || isRunning.value) return
  const msg = inputText.value.trim()
  inputText.value = ''
  messages.value.push({ role: 'user', blocks: [{ type: 'text', content: msg }] })
  isRunning.value = true
  streamingBlocks.value = []
  streamState.value = { inThinkTag: false, textIdx: -1 }
  lastSeq.value = 0
  scrollBottom()
  const s = getCurrentServer()
  sseXhr.value = sendMessageSSE({
    baseURL: s.baseURL, token: s.token,
    convId: currentConv.value.id, workspaceId: wsId.value,
    message: msg, lastSeq: 0,
    onEvent: e => handleEvent(e),
    onDone: () => { isRunning.value = false; flushStreaming() },
    onError: () => { if (isRunning.value) setTimeout(() => attachRunning(), 3000) }
  })
}

function handleEvent(e) {
  if (e.seq) lastSeq.value = e.seq
  const blocks = streamingBlocks.value
  const state = streamState.value

  switch (e.type) {
    case 'reasoning':
      appendReasoning(e.content || '')
      break
    case 'content':
      processContent(e.content || '')
      break
    case 'tool_call': {
      const name = e.data?.name || e.data?.function?.name || '工具'
      const args = formatArgs(e.data?.arguments || e.data?.args || e.data?.function?.arguments)
      blocks.push({ type: 'tool_call', name, args, collapsed: true })
      state.textIdx = -1
      break
    }
    case 'tool_result': {
      const isError = e.data?.is_error === true || e.data?.is_error === 'true'
      blocks.push({ type: 'tool_result', content: e.data?.content || '', isError, collapsed: true })
      state.textIdx = -1
      break
    }
    case 'error':
      blocks.push({ type: 'error', content: e.content || '未知错误' })
      break
    case 'status':
      messages.value.push({ role: 'status', blocks: [{ type: 'text', content: e.content || '' }] })
      break
    case 'token_update':
      if (e.data && e.data.total_tokens) {
        currentConv.value.total_tokens = e.data.total_tokens
      }
      break
    case 'done':
      if (e.data && e.data.total_tokens) {
        currentConv.value.total_tokens = e.data.total_tokens
      }
      isRunning.value = false
      flushStreaming()
      return
    default:
      return
  }
  scrollBottom()
}

function appendReasoning(text) {
  const blocks = streamingBlocks.value
  const last = blocks.length > 0 ? blocks[blocks.length - 1] : null
  if (last && last.type === 'reasoning') {
    last.content += text
  } else {
    blocks.push({ type: 'reasoning', content: text, collapsed: false })
  }
  streamState.value.textIdx = -1
}

function processContent(chunk) {
  const state = streamState.value
  let remaining = chunk
  while (remaining.length > 0) {
    if (state.inThinkTag) {
      const closeIdx = remaining.indexOf('</think>')
      if (closeIdx >= 0) {
        appendReasoning(remaining.slice(0, closeIdx))
        remaining = remaining.slice(closeIdx + 8)
        state.inThinkTag = false
      } else {
        appendReasoning(remaining)
        remaining = ''
      }
    } else {
      const openIdx = remaining.indexOf('<think>')
      if (openIdx >= 0) {
        if (openIdx > 0) appendText(remaining.slice(0, openIdx))
        state.inThinkTag = true
        remaining = remaining.slice(openIdx + 7)
      } else {
        appendText(remaining)
        remaining = ''
      }
    }
  }
}

function appendText(text) {
  const blocks = streamingBlocks.value
  const state = streamState.value
  if (state.textIdx >= 0 && state.textIdx < blocks.length && blocks[state.textIdx].type === 'text') {
    blocks[state.textIdx].content += text
  } else {
    state.textIdx = blocks.length
    blocks.push({ type: 'text', content: text })
  }
}

function flushStreaming() {
  if (streamingBlocks.value.length > 0) {
    for (const b of streamingBlocks.value) {
      if (b.type === 'reasoning') b.collapsed = true
    }
    messages.value.push({ role: 'assistant', blocks: [...streamingBlocks.value] })
    streamingBlocks.value = []
  }
  streamState.value = { inThinkTag: false, textIdx: -1 }
  scrollBottom()
}

async function stop() {
  await stopConversation(currentConv.value.id, wsId.value).catch(() => {})
  if (sseXhr.value) sseXhr.value.abort()
  isRunning.value = false
  flushStreaming()
}

function scrollBottom() {
  nextTick(() => { scrollToId.value = ''; nextTick(() => { scrollToId.value = 'm-end' }) })
}

function convStatusText(s) { return { 1: '进行中', 2: '已完成', 3: '已取消' }[s] || '' }
</script>

<style scoped>
.page { display: flex; flex-direction: column; overflow: hidden; }

/* 自定义导航栏 */
.nav-bar { display: flex; align-items: center; padding: 0 20rpx; height: 44px; background: #007aff; padding-top: var(--status-bar-height); }
.nav-back { font-size: 48rpx; color: #fff; width: 60rpx; }
.nav-title { flex: 1; font-size: 34rpx; font-weight: 500; color: #fff; text-align: center; overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.nav-action { font-size: 28rpx; color: #fff; }
.nav-tasks { font-size: 26rpx; color: #fff; margin-right: 20rpx; }
.nav-stop { font-size: 26rpx; color: #ffe0e0; }

/* 会话列表 */
.conv-view { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.conv-scroll { flex: 1; height: 0; }
.empty { text-align: center; padding: 120rpx 30rpx; color: #999; font-size: 28rpx; }
.list-block { background: #fff; }
.list-item { display: flex; align-items: center; padding: 22rpx 30rpx; position: relative; }
.list-item::after { content: ''; position: absolute; bottom: 0; left: 30rpx; right: 0; height: 1px; background: #e5e5e5; transform: scaleY(0.5); }
.list-item:last-child::after { display: none; }
.item-body { flex: 1; }
.item-title { font-size: 30rpx; color: #333; display: block; margin-bottom: 6rpx; }
.item-tokens { font-size: 22rpx; color: #bbb; }
.conv-badge { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 20rpx; margin-right: 12rpx; }
.s1 { background: #e3f2fd; color: #007aff; }
.s2 { background: #e8f5e9; color: #34c759; }
.s3 { background: #f5f5f5; color: #999; }
.arrow { font-size: 36rpx; color: #c8c7cc; }

/* 对话视图 */
.chat-view { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.messages { flex: 1; height: 0; padding: 20rpx 0; background: #efeff4; }
.msg-row { display: flex; margin-bottom: 20rpx; padding: 0 24rpx; }
.msg-row.user { justify-content: flex-end; }
.msg-row.assistant { justify-content: flex-start; }
.msg-row.status { justify-content: center; }

/* 气泡 */
.bubble { max-width: 80%; padding: 20rpx 24rpx; border-radius: 24rpx; word-break: break-all; }
.bubble.user { background: #007aff; border-bottom-right-radius: 6rpx; }
.bubble.assistant { background: #fff; border-bottom-left-radius: 6rpx; box-shadow: 0 2rpx 8rpx rgba(0,0,0,0.04); }
.bubble.streaming { opacity: 0.9; }

/* 文本块 */
.msg-text { font-size: 28rpx; line-height: 1.6; color: #333; }
.bubble.user .msg-text { color: #fff; }
.typing-dot { color: #999; font-size: 36rpx; }

/* 状态消息 */
.status-notice { padding: 10rpx 0; }
.status-text { font-size: 22rpx; color: #9ca3af; background: #f3f4f6; padding: 6rpx 20rpx; border-radius: 20rpx; }

/* 思考过程块 */
.block-reasoning { margin: 12rpx 0; padding: 14rpx 18rpx; border-left: 4rpx solid #a78bfa; background: #f5f3ff; border-radius: 8rpx; }
.block-header { display: flex; align-items: center; }
.block-icon { font-size: 24rpx; margin-right: 8rpx; }
.block-label { font-size: 24rpx; font-weight: 500; flex: 1; }
.reasoning-label { color: #7c3aed; }
.block-toggle { font-size: 22rpx; color: #a78bfa; }
.reasoning-body { font-size: 26rpx; color: #6b7280; margin-top: 10rpx; line-height: 1.5; white-space: pre-wrap; word-break: break-all; }
.streaming-reason { max-height: 200rpx; overflow-y: auto; }

/* 工具调用块 */
.block-tool-call { margin: 12rpx 0; padding: 14rpx 18rpx; border-left: 4rpx solid #3b82f6; background: #eff6ff; border-radius: 8rpx; }
.tool-call-label { color: #1d4ed8; }
.tool-call-args { margin-top: 10rpx; padding: 12rpx; background: #f8fafc; border-radius: 6rpx; max-height: 400rpx; overflow-y: auto; }

/* 工具结果块 */
.block-tool-result { margin: 12rpx 0; padding: 14rpx 18rpx; border-left: 4rpx solid #22c55e; background: #f0fdf4; border-radius: 8rpx; }
.block-tool-result.is-error { border-left-color: #ef4444; background: #fef2f2; }
.result-icon { font-size: 26rpx; font-weight: 700; margin-right: 8rpx; }
.icon-ok { color: #22c55e; }
.icon-err { color: #ef4444; }
.result-preview { font-size: 24rpx; color: #6b7280; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.is-error .result-preview { color: #dc2626; }
.tool-result-body { margin-top: 10rpx; padding: 12rpx; background: #f8fafc; border-radius: 6rpx; max-height: 600rpx; overflow-y: auto; }

/* 错误块 */
.block-error { margin: 12rpx 0; padding: 14rpx 18rpx; background: #fef2f2; border-radius: 8rpx; border-left: 4rpx solid #ef4444; }
.error-text { font-size: 26rpx; color: #dc2626; line-height: 1.5; }

/* 代码块 */
.block-code { margin: 12rpx 0; padding: 16rpx; background: #1e293b; border-radius: 10rpx; overflow-x: auto; }
.code-lang { font-size: 20rpx; color: #94a3b8; margin-bottom: 8rpx; display: block; }
.code-text { font-size: 24rpx; color: #e2e8f0; font-family: Menlo, Consolas, monospace; line-height: 1.5; white-space: pre-wrap; word-break: break-all; }
.tool-call-args .code-text { color: #374151; font-size: 22rpx; }
.tool-result-body .code-text { color: #374151; font-size: 22rpx; }

/* 输入栏 */
.input-bar { display: flex; align-items: flex-end; padding: 12rpx 20rpx; background: #fff; border-top: 1px solid #e5e5e5; }
.input { flex: 1; background: #f7f7f7; border-radius: 40rpx; padding: 16rpx 24rpx; font-size: 28rpx; min-height: 72rpx; max-height: 200rpx; }
.send-btn { padding: 16rpx 20rpx 16rpx 16rpx; font-size: 30rpx; color: #c8c7cc; }
.send-btn.active { color: #007aff; }
</style>
