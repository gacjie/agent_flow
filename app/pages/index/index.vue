<template>
  <view class="page">
    <view class="section-title">
      <text class="section-label">我的服务器</text>
      <text class="add-btn" @click="showAdd = true">+ 添加</text>
    </view>

    <view v-if="servers.length === 0" class="empty"><text>暂无服务器，点击右上角添加</text></view>

    <view v-else class="list-block">
      <view v-for="s in servers" :key="s.id" class="list-item" @click="enter(s)">
        <view class="item-body">
          <text class="item-name">{{ s.name }}</text>
          <text class="item-url">{{ s.baseURL }}</text>
        </view>
        <text class="status-badge" :class="s.token ? 'ok' : 'unauth'">{{ s.token ? '已登录' : '未登录' }}</text>
        <text class="del-btn" @click.stop="remove(s.id)">删除</text>
        <text class="arrow">›</text>
      </view>
    </view>

    <view v-if="showAdd" class="modal-wrap">
      <view class="modal-mask" @click="showAdd = false"></view>
      <view class="modal-box">
        <text class="modal-title">添加服务器</text>
        <view class="form-field">
          <input :value="form.name" @input="form.name = $event.detail.value" placeholder="名称（如：工作服务器）" class="field-input" cursor-spacing="20" />
        </view>
        <view class="form-field">
          <input :value="form.baseURL" @input="form.baseURL = $event.detail.value" placeholder="地址（如：http://192.168.1.1:8080）" class="field-input" cursor-spacing="20" />
        </view>
        <view class="modal-btns">
          <view class="btn-cancel" @click="showAdd = false">取消</view>
          <view class="btn-primary" @click="addSrv">确定</view>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { getServers, addServer, removeServer, setCurrentServer } from '../../utils/storage.js'

const servers = ref([])
const showAdd = ref(false)
const form = ref({ name: '', baseURL: '' })

onShow(() => { servers.value = getServers() })

function addSrv() {
  if (!form.value.name || !form.value.baseURL) return uni.showToast({ title: '请填写完整', icon: 'none' })
  addServer({ name: form.value.name, baseURL: form.value.baseURL.replace(/\/$/, ''), token: null })
  form.value = { name: '', baseURL: '' }
  showAdd.value = false
  servers.value = getServers()
}

function remove(id) {
  uni.showModal({ title: '确认删除？', success: r => { if (r.confirm) { removeServer(id); servers.value = getServers() } } })
}

function enter(server) {
  setCurrentServer(server)
  if (!server.token) {
    uni.navigateTo({ url: `/pages/login/login?serverId=${server.id}` })
  } else {
    uni.navigateTo({ url: '/pages/workbench/workbench' })
  }
}
</script>

<style scoped>
.page { padding: 0 0 40rpx; }
.section-title { display: flex; justify-content: space-between; align-items: center; padding: 24rpx 30rpx 16rpx; }
.section-label { font-size: 26rpx; color: #999; }
.add-btn { font-size: 28rpx; color: #007aff; }
.empty { text-align: center; padding: 120rpx 30rpx; color: #999; font-size: 28rpx; }
.list-block { background: #fff; }
.list-item { display: flex; align-items: center; padding: 24rpx 30rpx; position: relative; }
.list-item::after { content: ''; position: absolute; bottom: 0; left: 30rpx; right: 0; height: 1px; background: #e5e5e5; transform: scaleY(0.5); }
.list-item:last-child::after { display: none; }
.item-body { flex: 1; }
.item-name { font-size: 32rpx; color: #333; font-weight: 500; display: block; margin-bottom: 6rpx; }
.item-url { font-size: 24rpx; color: #999; display: block; }
.status-badge { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 20rpx; margin-right: 16rpx; }
.ok { background: #e8f5e9; color: #34c759; }
.unauth { background: #fff3e0; color: #ff9500; }
.del-btn { font-size: 26rpx; color: #dd524d; margin-right: 16rpx; }
.arrow { font-size: 36rpx; color: #c8c7cc; }
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
