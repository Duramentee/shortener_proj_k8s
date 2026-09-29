<script setup>
// 创建表单组件。
// 本组件只负责收集用户输入与做最基本的空值检查，其余校验（是否以 http:// 或 https:// 开头、
// 长度是否超过 2048）全部由后端完成，前端把后端返回的 error 字段原样显示出来。
// 这样做的原因是：校验规则只能有一份权威实现，前端重复实现一份会出现两边规则不一致的情况。
import { ref } from 'vue'

defineProps({
  /** 请求是否正在进行中，为真时禁用输入框与按钮。 */
  busy: { type: Boolean, default: false },
  /** 由父组件传入的、来自后端的错误信息。 */
  submitError: { type: String, default: '' }
})

const emit = defineEmits(['submit'])

const url = ref('')
const localError = ref('')

function handleSubmit() {
  const value = url.value.trim()
  if (value === '') {
    localError.value = '请输入需要缩短的长网址'
    return
  }
  localError.value = ''
  emit('submit', value)
}

/** 由父组件在创建成功之后调用，用于清空输入框与本地错误提示。 */
function reset() {
  url.value = ''
  localError.value = ''
}

/** 供父组件在需要时预填一个网址，例如以后要做的「示例链接」按钮。 */
function fill(value) {
  url.value = value
}

defineExpose({ reset, fill })
</script>

<template>
  <form class="card" @submit.prevent="handleSubmit">
    <label class="field-label" for="long-url">长网址</label>
    <div class="field-row">
      <input
        id="long-url"
        v-model="url"
        type="text"
        name="url"
        autocomplete="off"
        spellcheck="false"
        placeholder="https://example.com/very/long/path"
        :disabled="busy"
      />
      <button type="submit" :disabled="busy">
        {{ busy ? '正在创建' : '生成短链接' }}
      </button>
    </div>
    <p v-if="localError || submitError" class="error-text">{{ localError || submitError }}</p>
  </form>
</template>
