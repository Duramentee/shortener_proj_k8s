<script setup>
// 后端状态指示器，对应接口契约中的 GET /api/readyz。
//
// 本组件的存在意义与探针有关：就绪探针会返回 503 并且把每个依赖项的状态写在 checks 字段里，
// 界面上把 checks 的内容原样显示出来，可以在演示就绪探针实验时直接看到「哪一个依赖不可用」，
// 不需要每次都到终端里执行 curl 命令。
import { computed } from 'vue'

const props = defineProps({
  /** 形如 { ok: boolean, status: number, body: object|null }，由 api.js 的 fetchReadiness 返回。 */
  readiness: {
    type: Object,
    required: true,
    default: () => ({ ok: false, status: 0, body: null })
  }
})

defineEmits(['refresh'])

/** 状态文字。status 为 0 表示连 HTTP 响应都没有收到，也就是网络层就失败了。 */
const stateText = computed(() => {
  if (props.readiness.ok) {
    return '后端就绪'
  }
  if (props.readiness.status === 0) {
    return '无法连接后端'
  }
  return `后端未就绪，HTTP 状态码是 ${props.readiness.status}`
})

/** 把 checks 对象展开成「postgres=ok，redis=ok」这样的可读文本。 */
const checksText = computed(() => {
  const checks = props.readiness.body?.checks
  if (checks === undefined || checks === null || typeof checks !== 'object') {
    return ''
  }
  return Object.entries(checks)
    .map(([name, value]) => `${name}=${value}`)
    .join('，')
})
</script>

<template>
  <div class="status" :class="readiness.ok ? 'status-ok' : 'status-bad'">
    <span class="status-dot" aria-hidden="true"></span>
    <span class="status-text">
      {{ stateText }}
      <span v-if="checksText" class="status-checks">（{{ checksText }}）</span>
    </span>
    <button type="button" class="ghost" @click="$emit('refresh')">刷新状态</button>
  </div>
</template>
