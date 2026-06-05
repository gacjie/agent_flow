<template>
  <view class="page">
    <view class="section-title">
      <text class="section-label">任务列表</text>
    </view>
    <view v-if="loading" class="empty"><text>加载中...</text></view>
    <view v-else-if="tasks.length === 0" class="empty"><text>暂无任务</text></view>
    <view v-else class="list-block">
      <view v-for="t in tasks" :key="t.id" class="list-item">
        <view :class="'status-dot s' + t.status"></view>
        <view class="item-body">
          <text class="item-title">{{ t.title }}</text>
          <text class="item-phase">阶段 {{ t.phase }}{{ t.phase_label ? ' · ' + t.phase_label : '' }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup>
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { listTasks } from '../../utils/api.js'

const wsId = ref(null)
const tasks = ref([])
const loading = ref(false)

onLoad((opts) => {
  wsId.value = parseInt(opts.wsId)
  load()
})

async function load() {
  loading.value = true
  const res = await listTasks(wsId.value).catch(() => null)
  tasks.value = res?.data?.tasks || []
  loading.value = false
}
</script>

<style scoped>
.page { padding: 0 0 40rpx; }
.section-title { padding: 24rpx 30rpx 16rpx; }
.section-label { font-size: 26rpx; color: #999; }
.empty { text-align: center; padding: 120rpx 30rpx; color: #999; font-size: 28rpx; }
.list-block { background: #fff; }
.list-item { display: flex; align-items: center; padding: 22rpx 30rpx; position: relative; }
.list-item::after { content: ''; position: absolute; bottom: 0; left: 30rpx; right: 0; height: 1px; background: #e5e5e5; transform: scaleY(0.5); }
.list-item:last-child::after { display: none; }
.status-dot { width: 16rpx; height: 16rpx; border-radius: 8rpx; margin-right: 20rpx; }
.s0 { background: #c8c7cc; }
.s1 { background: #007aff; }
.s2 { background: #34c759; }
.s3 { background: #dd524d; }
.s4 { background: #c8c7cc; }
.item-body { flex: 1; }
.item-title { font-size: 28rpx; color: #333; display: block; margin-bottom: 6rpx; }
.item-phase { font-size: 22rpx; color: #999; }
</style>
