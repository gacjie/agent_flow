<template>
  <view class="page">
    <view class="logo-area">
      <text class="logo-icon">⚡</text>
      <text class="logo-title">AgentFlow</text>
      <text class="logo-sub">登录到 {{ serverName }}</text>
    </view>

    <view class="form-block">
      <view class="form-row">
        <text class="form-label">账号</text>
        <input :value="form.username" @input="form.username = $event.detail.value" placeholder="请输入用户名" class="form-input" cursor-spacing="20" />
      </view>
      <view class="divider"></view>
      <view class="form-row">
        <text class="form-label">密码</text>
        <input :value="form.password" @input="form.password = $event.detail.value" placeholder="请输入密码" :password="true" class="form-input" cursor-spacing="20" />
      </view>
    </view>

    <view class="btn-wrap">
      <button class="btn-login" :disabled="loading" @click="login">{{ loading ? '登录中...' : '登录' }}</button>
    </view>

    <text v-if="error" class="error-msg">{{ error }}</text>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { apiLogin } from '../../utils/api.js'
import { getServers, updateServer, setCurrentServer } from '../../utils/storage.js'

const serverId = ref('')
const serverName = ref('')
const form = ref({ username: '', password: '' })
const loading = ref(false)
const error = ref('')

onLoad((opts) => {
  serverId.value = opts.serverId
  const s = getServers().find(s => s.id === opts.serverId)
  if (s) serverName.value = s.name
})

async function login() {
  if (!form.value.username || !form.value.password) return
  loading.value = true; error.value = ''
  try {
    const s = getServers().find(s => s.id === serverId.value)
    const res = await apiLogin(s.baseURL, form.value.username, form.value.password)
    updateServer(serverId.value, { token: res.token, adminName: res.admin_name })
    setCurrentServer({ ...s, token: res.token, adminName: res.admin_name })
    uni.redirectTo({ url: '/pages/workbench/workbench' })
  } catch (e) {
    error.value = e?.message || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.page { padding: 0 30rpx; }
.logo-area { display: flex; flex-direction: column; align-items: center; padding: 100rpx 0 60rpx; }
.logo-icon { font-size: 80rpx; margin-bottom: 16rpx; }
.logo-title { font-size: 44rpx; font-weight: bold; color: #333; margin-bottom: 12rpx; }
.logo-sub { font-size: 26rpx; color: #999; }
.form-block { background: #fff; border-radius: 16rpx; overflow: hidden; margin-bottom: 30rpx; }
.form-row { display: flex; align-items: center; padding: 0 30rpx; height: 96rpx; }
.form-label { font-size: 30rpx; color: #333; width: 100rpx; }
.form-input { flex: 1; font-size: 30rpx; color: #333; height: 96rpx; }
.divider { height: 1px; background: #e5e5e5; margin-left: 30rpx; transform: scaleY(0.5); }
.btn-wrap { margin-top: 10rpx; }
.btn-login { background: #007aff; color: #fff; border-radius: 50rpx; font-size: 34rpx; height: 96rpx; line-height: 96rpx; border: none; }
.btn-login[disabled] { opacity: 0.6; }
.error-msg { display: block; text-align: center; color: #dd524d; font-size: 26rpx; margin-top: 20rpx; }
</style>
