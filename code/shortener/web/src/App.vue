<script setup>
// 根组件：持有页面的全部状态，并且负责调用后端接口。
// 子组件只负责展示与触发事件，不直接访问接口，这样数据的来源只有一个，排查问题时只需要看本文件。
import { computed, onMounted, ref } from 'vue'
import { createLink, deleteLink, fetchReadiness, fetchStats, listLinks } from './api.js'
import CreateLinkForm from './components/CreateLinkForm.vue'
import LinkTable from './components/LinkTable.vue'
import Pagination from './components/Pagination.vue'
import StatsBar from './components/StatsBar.vue'
import StatusIndicator from './components/StatusIndicator.vue'

/** 每页显示的记录条数。接口契约规定 limit 的上限是 100，这里取 20。 */
const LIMIT = 20

/** 创建表单组件的引用，用于在创建成功之后清空输入框。 */
const createForm = ref(null)

const links = ref([])
const total = ref(0)
const offset = ref(0)
const stats = ref(null)
const readiness = ref({ ok: false, status: 0, body: null })

const listLoading = ref(false)
const statsLoading = ref(false)
const creating = ref(false)
const deletingCode = ref('')

const errorMessage = ref('')
const createError = ref('')
const noticeMessage = ref('')

const hasPrevPage = computed(() => offset.value > 0)
const hasNextPage = computed(() => offset.value + LIMIT < total.value)

/** 加载当前页的短链接列表。失败时清空列表并且把原因显示在页面顶部的错误提示条上。 */
async function reloadLinks() {
  listLoading.value = true
  try {
    const data = await listLinks({ limit: LIMIT, offset: offset.value })
    links.value = Array.isArray(data?.items) ? data.items : []
    total.value = typeof data?.total === 'number' ? data.total : 0
  } catch (error) {
    links.value = []
    total.value = 0
    errorMessage.value = `加载短链接列表失败：${error.message}`
  } finally {
    listLoading.value = false
  }
}

/** 加载汇总统计。 */
async function reloadStats() {
  statsLoading.value = true
  try {
    stats.value = await fetchStats()
  } catch (error) {
    stats.value = null
    errorMessage.value = `加载统计数据失败：${error.message}`
  } finally {
    statsLoading.value = false
  }
}

/**
 * 查询后端就绪状态。这个函数不会抛出异常，
 * 因为就绪探针返回 503 本身就是一种需要展示出来的正常结果。
 */
async function reloadReadiness() {
  readiness.value = await fetchReadiness()
}

/** 三个数据源一起刷新，用于首次加载、手动刷新以及每次改动数据之后。 */
async function reloadAll() {
  await Promise.all([reloadLinks(), reloadStats(), reloadReadiness()])
}

/** 处理创建表单提交的事件：调用接口，成功后清空输入框并且回到第一页。 */
async function handleCreate(url) {
  creating.value = true
  createError.value = ''
  try {
    const created = await createLink(url)
    noticeMessage.value = `已创建短链接 ${window.location.origin}/${created.code}`
    // 调用子组件通过 defineExpose 暴露出来的方法清空输入框。
    createForm.value?.reset()
    offset.value = 0
    await reloadAll()
  } catch (error) {
    createError.value = error.message
  } finally {
    creating.value = false
  }
}

/** 处理删除事件。删除当前页的最后一条记录时，页码回退一页，避免停留在空白页。 */
async function handleDelete(code) {
  deletingCode.value = code
  try {
    await deleteLink(code)
    noticeMessage.value = `已删除短链接 /${code}`
    if (links.value.length === 1 && offset.value > 0) {
      offset.value = Math.max(0, offset.value - LIMIT)
    }
    await reloadAll()
  } catch (error) {
    errorMessage.value = `删除短链接 /${code} 失败：${error.message}`
  } finally {
    deletingCode.value = ''
  }
}

/**
 * 复制短链接到剪贴板。
 * 浏览器只有在安全上下文中才允许写入剪贴板，安全上下文包括 https 与 localhost；
 * 通过 http 加上集群节点 IP 访问时该接口不可用，因此这里必须处理失败的情况并且提示用户手动复制。
 */
async function handleCopy(code) {
  const text = `${window.location.origin}/${code}`
  try {
    await navigator.clipboard.writeText(text)
    noticeMessage.value = `已复制 ${text}`
  } catch (copyError) {
    noticeMessage.value = `浏览器不允许写入剪贴板，请手动复制 ${text}`
  }
}

function handlePrevPage() {
  if (!hasPrevPage.value) {
    return
  }
  offset.value = Math.max(0, offset.value - LIMIT)
  reloadLinks()
}

function handleNextPage() {
  if (!hasNextPage.value) {
    return
  }
  offset.value += LIMIT
  reloadLinks()
}

onMounted(reloadAll)
</script>

<template>
  <div class="page">
    <header class="page-header">
      <div>
        <h1>shortener · 短链接服务</h1>
        <p class="subtitle">Go 后端、PostgreSQL 与 Redis，前端由 nginx 提供，用于部署与排障练习</p>
      </div>
      <StatusIndicator :readiness="readiness" @refresh="reloadReadiness" />
    </header>

    <p v-if="errorMessage" class="banner banner-error">
      <span>{{ errorMessage }}</span>
      <button type="button" class="banner-close" @click="errorMessage = ''">关闭提示</button>
    </p>

    <p v-if="noticeMessage" class="banner banner-ok">
      <span>{{ noticeMessage }}</span>
      <button type="button" class="banner-close" @click="noticeMessage = ''">关闭提示</button>
    </p>

    <StatsBar :stats="stats" :loading="statsLoading" />

    <CreateLinkForm
      ref="createForm"
      :busy="creating"
      :submit-error="createError"
      @submit="handleCreate"
    />

    <section class="card">
      <div class="card-header">
        <h2>已创建的短链接</h2>
        <button type="button" class="ghost" :disabled="listLoading" @click="reloadAll">重新加载全部数据</button>
      </div>

      <LinkTable
        :items="links"
        :loading="listLoading"
        :deleting-code="deletingCode"
        @delete="handleDelete"
        @copy="handleCopy"
      />

      <Pagination
        :total="total"
        :limit="LIMIT"
        :offset="offset"
        :disabled="listLoading"
        @prev="handlePrevPage"
        @next="handleNextPage"
      />
    </section>

    <footer class="page-footer">
      <span>接口契约见 code/shortener/README.md 第 4 节</span>
    </footer>
  </div>
</template>
