<script setup>
// 统计条组件，对应接口契约中的 GET /api/stats。
// 四个数据项的含义分别是：短链接总数、累计点击数、缓存命中率、缓存命中与未命中的原始次数。
// 同时显示命中率与原始次数，原因是命中率是计算得出的比例，原始次数能说明这个比例的样本量大小。
import { computed } from 'vue'

const props = defineProps({
  /** 统计对象，尚未加载完成时为 null。 */
  stats: { type: Object, default: null },
  /** 是否正在加载。 */
  loading: { type: Boolean, default: false }
})

/** 命中率在接口中是小数值（例如 0.9），这里换算成百分比并且保留一位小数。 */
const hitRateText = computed(() => {
  const rate = props.stats?.cacheHitRate
  if (typeof rate !== 'number' || Number.isNaN(rate)) {
    return '暂无数据'
  }
  return `${(rate * 100).toFixed(1)}%`
})

/** 读取统计项的取值，字段缺失时统一显示为「暂无数据」。 */
function valueOf(key) {
  const value = props.stats?.[key]
  if (value === undefined || value === null) {
    return '暂无数据'
  }
  return String(value)
}
</script>

<template>
  <section class="stats-bar">
    <div class="stat-card">
      <span class="stat-label">短链接总数</span>
      <span class="stat-value">{{ valueOf('links') }}</span>
    </div>
    <div class="stat-card">
      <span class="stat-label">累计点击数</span>
      <span class="stat-value">{{ valueOf('clicks') }}</span>
    </div>
    <div class="stat-card">
      <span class="stat-label">缓存命中率</span>
      <span class="stat-value">{{ hitRateText }}</span>
    </div>
    <div class="stat-card">
      <span class="stat-label">缓存命中与未命中次数</span>
      <span class="stat-value">{{ valueOf('cacheHits') }} / {{ valueOf('cacheMisses') }}</span>
    </div>
    <p v-if="loading" class="stat-loading">正在加载统计数据</p>
  </section>
</template>
