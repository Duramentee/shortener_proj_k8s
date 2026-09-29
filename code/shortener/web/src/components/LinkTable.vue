<script setup>
// 列表组件。本组件只负责展示数据与触发事件，不发起任何请求。
// delete 事件只在用户点击删除按钮时触发，真正的删除请求由父组件发起；
// copy 事件只在用户点击复制按钮时触发，真正的剪贴板写入也由父组件完成。
defineProps({
  /** 当前页的短链接记录，每一项包含 code、url、clicks、createdAt 四个字段。 */
  items: { type: Array, default: () => [] },
  /** 加载状态。为真时显示加载提示而不是空列表提示，避免用户误以为没有数据。 */
  loading: { type: Boolean, default: false },
  /** 正在被删除的短码。与某一行的 code 相同时，该行的删除按钮显示为禁用状态。 */
  deletingCode: { type: String, default: '' }
})

defineEmits(['delete', 'copy'])

/** 把后端返回的时间戳格式化为本地时间字符串。取值无法解析时退回到原始字符串。 */
function formatTime(value) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }
  return date.toLocaleString('zh-CN', { hour12: false })
}
</script>

<template>
  <div class="table-wrapper">
    <p v-if="loading" class="table-hint">正在加载短链接列表</p>
    <p v-else-if="items.length === 0" class="table-hint">
      当前没有短链接记录，请在上方输入一个长网址并点击生成短链接。
    </p>
    <table v-else class="link-table">
      <thead>
        <tr>
          <th>短码</th>
          <th>原始长网址</th>
          <th>点击数</th>
          <th>创建时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="item in items" :key="item.code">
          <td>
            <!-- 使用 href 而不是脚本跳转，这样浏览器会真实发起一次 GET /{code} 请求，
                 由后端返回 302 跳转，与实际用户的使用路径完全一致。 -->
            <a :href="`/${item.code}`" target="_blank" rel="noopener">{{ item.code }}</a>
          </td>
          <td class="url-cell" :title="item.url">{{ item.url }}</td>
          <td>{{ item.clicks }}</td>
          <td>{{ formatTime(item.createdAt) }}</td>
          <td class="action-cell">
            <button type="button" class="ghost" @click="$emit('copy', item.code)">复制短链接</button>
            <button
              type="button"
              class="danger"
              :disabled="deletingCode === item.code"
              @click="$emit('delete', item.code)"
            >
              {{ deletingCode === item.code ? '正在删除' : '删除' }}
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
