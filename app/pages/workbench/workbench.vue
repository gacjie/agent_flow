<template>
  <view class="page">
    <view class="server-bar">
      <text class="server-name">{{ serverName }}</text>
      <text class="logout-btn" @click="logout">退出登录</text>
    </view>

    <view class="section-title">
      <text class="section-label">工作区</text>
    </view>

    <view v-if="loading" class="empty"><text>加载中...</text></view>
    <view v-else-if="workspaces.length === 0" class="empty"><text>暂无工作区，点击 + 新建</text></view>

    <view v-else class="list-block">
      <view v-for="ws in workspaces" :key="ws.id" class="list-item" @click="enterWorkspace(ws)">
        <view class="item-body">
          <text class="item-name">{{ ws.label || ws.name }}</text>
          <text class="item-desc">{{ ws.description || '暂无描述' }}</text>
          <text class="item-agent">{{ ws.Agent ? ws.Agent.title : '未绑定 Agent' }}</text>
        </view>
        <text class="ws-badge" :class="'s' + ws.status">{{ statusText(ws.status) }}</text>
        <text class="arrow">›</text>
      </view>
    </view>

    <view class="fab" @click="showCreate = true"><text class="fab-icon">+</text></view>

    <view v-if="showCreate" class="modal-wrap">
      <view class="modal-mask" @click="showCreate = false"></view>
      <view class="modal-box">
        <text class="modal-title">新建工作区</text>
        <view class="form-field">
          <input :value="newWs.name" @input="newWs.name = $event.detail.value" placeholder="工作区名称" class="field-input" cursor-spacing="20" />
        </view>
        <view class="form-field">
          <input :value="newWs.description" @input="newWs.description = $event.detail.value" placeholder="描述（可选）" class="field-input" cursor-spacing="20" />
        </view>
        <view class="modal-btns">
          <view class="btn-cancel" @click="showCreate = false">取消</view>
          <view class="btn-primary" @click="createWs">创建</view>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { getCurrentServer, setCurrentServer } from '../../utils/storage.js'
import { request, apiLogout } from '../../utils/api.js'

const serverName = ref('')
const workspaces = ref([])
const loading = ref(false)
const showCreate = ref(false)
const newWs = ref({ name: '', description: '' })

onShow(() => {
  const s = getCurrentServer()
  if (!s || !s.token) return uni.redirectTo({ url: '/pages/index/index' })
  serverName.value = s.name
  load()
})

async function load() {
  loading.value = true
  try {
    const res = await request('GET', '/admin/workbench/workspaces')
    workspaces.value = res.data || []
  } catch {
    workspaces.value = []
  } finally {
    loading.value = false
  }
}

async function createWs() {
  if (!newWs.value.name) return
  await request('POST', '/admin/workbench/workspaces', { name: newWs.value.name, label: newWs.value.name, description: newWs.value.description })
  newWs.value = { name: '', description: '' }
  showCreate.value = false
  load()
}

function enterWorkspace(ws) {
  uni.navigateTo({ url: `/pages/chat/chat?wsId=${ws.id}&wsName=${encodeURIComponent(ws.label || ws.name)}` })
}

async function logout() {
  await apiLogout().catch(() => {})
  const s = getCurrentServer()
  if (s) { s.token = null; setCurrentServer(s) }
  uni.redirectTo({ url: '/pages/index/index' })
}

function statusText(s) { return { 1: '进行中', 2: '已完成', 3: '已暂停' }[s] || '' }
</script>

<style scoped>
.page { padding: 0 0 120rpx; }
.server-bar { display: flex; justify-content: space-between; align-items: center; padding: 20rpx 30rpx; background: #fff; border-bottom: 1px solid #e5e5e5; }
.server-name { font-size: 26rpx; color: #666; }
.logout-btn { font-size: 26rpx; color: #dd524d; }
.section-title { padding: 24rpx 30rpx 16rpx; }
.section-label { font-size: 26rpx; color: #999; }
.empty { text-align: center; padding: 120rpx 30rpx; color: #999; font-size: 28rpx; }
.list-block { background: #fff; }
.list-item { display: flex; align-items: center; padding: 24rpx 30rpx; position: relative; }
.list-item::after { content: ''; position: absolute; bottom: 0; left: 30rpx; right: 0; height: 1px; background: #e5e5e5; transform: scaleY(0.5); }
.list-item:last-child::after { display: none; }
.item-body { flex: 1; }
.item-name { font-size: 32rpx; color: #333; font-weight: 500; display: block; margin-bottom: 6rpx; }
.item-desc { font-size: 24rpx; color: #999; display: block; margin-bottom: 4rpx; }
.item-agent { font-size: 22rpx; color: #007aff; }
.ws-badge { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 20rpx; margin-right: 12rpx; }
.s1 { background: #e3f2fd; color: #007aff; }
.s2 { background: #e8f5e9; color: #34c759; }
.s3 { background: #f5f5f5; color: #999; }
.arrow { font-size: 36rpx; color: #c8c7cc; }
.fab { position: fixed; bottom: 60rpx; right: 40rpx; width: 100rpx; height: 100rpx; background: #007aff; border-radius: 50rpx; box-shadow: 0 4rpx 16rpx rgba(0,122,255,0.4); display: flex; align-items: center; justify-content: center; }
.fab-icon { color: #fff; font-size: 60rpx; line-height: 1; }
.modal-wrap { position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 999; }
.modal-mask { position: absolute; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.4); }
.modal-box { position: absolute; top: 50%; left: 50%; transform: translate(-50%, -50%); background: #fff; border-radius: 20rpx; padding: 40rpx; width: 620rpx; }
.modal-title { font-size: 34rpx; font-weight: bold; color: #333; display: block; margin-bottom: 30rpx; }
.form-field { background: #f7f7f7; border-radius: 10rpx; padding: 0 20rpx; margin-bottom: 20rpx; }
.field-input { height: 88rpx; font-size: 28rpx; color: #333; }
.modal-btns { display: flex; margin-top: 10rpx; }
.btn-cancel { flex: 1; background: #f7f7f7; color: #333; border-radius: 10rpx; height: 88rpx; line-height: 88rpx; text-align: center; font-size: 30rpx; margin-right: 20rpx; }
.btn-primary { flex: 1; background: #007aff; color: #fff; border-radius: 10rpx; height: 88rpx; line-height: 88rpx; text-align: center; font-size: 30rpx; }
</style>
