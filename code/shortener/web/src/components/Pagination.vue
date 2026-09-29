<script setup>
// 分页组件。偏移量的计算与接口调用都由父组件负责，本组件只根据 total、limit、offset 三个输入
// 判断按钮是否可用，并且把用户点击的意图通过事件抛给父组件。
import { computed } from 'vue'

const props = defineProps({
  /** 记录总数，来自 GET /api/links 响应体中的 total 字段。 */
  total: { type: Number, default: 0 },
  /** 每页条数，与父组件请求接口时使用的 limit 保持一致。 */
  limit: { type: Number, default: 20 },
  /** 当前偏移量。 */
  offset: { type: Number, default: 0 },
  /** 列表是否正在加载，为真时两个按钮都不可用，防止连点导致页码错乱。 */
  disabled: { type: Boolean, default: false }
})

defineEmits(['prev', 'next'])

/** 当前页显示的是第几条到第几条，例如「第 21 至 40 条，共 42 条记录」。 */
const rangeText = computed(() => {
  if (props.total === 0) {
    return '共 0 条记录'
  }
  const first = props.offset + 1
  const last = Math.min(props.offset + props.limit, props.total)
  return `第 ${first} 至 ${last} 条，共 ${props.total} 条记录`
})

const canPrev = computed(() => props.offset > 0)
const canNext = computed(() => props.offset + props.limit < props.total)
</script>

<template>
  <div class="pagination">
    <span class="pagination-text">{{ rangeText }}</span>
    <div class="pagination-buttons">
      <button type="button" class="ghost" :disabled="disabled || !canPrev" @click="$emit('prev')">
        上一页
      </button>
      <button type="button" class="ghost" :disabled="disabled || !canNext" @click="$emit('next')">
        下一页
      </button>
    </div>
  </div>
</template>
