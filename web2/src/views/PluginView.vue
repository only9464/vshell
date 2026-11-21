<script setup lang="ts">
// 插件：列出 plugins/ 下的可执行文件，选中执行并回显输出
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { get, post } from '../api/http'

const route = useRoute()
const id = Number(route.params.id) || 0
const plugins = ref<{ id: number; name: string }[]>([])
const filter = ref('')
const procArg = ref('')
const output = ref('')
const loading = ref(false)

const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return q ? plugins.value.filter((p) => p.name.toLowerCase().includes(q)) : plugins.value
})

async function load() {
  const res: any = await get('/runner/list')
  plugins.value = res?.result || []
}
onMounted(load)

async function run(name: string) {
  loading.value = true
  output.value = ''
  try {
    const res: any = await post('/runner/runplugin', { id, pluginName: name, procArg: procArg.value })
    output.value = res?.result?.output ?? JSON.stringify(res, null, 2)
  } catch (e: any) {
    output.value = e?.message || String(e)
  } finally { loading.value = false }
}

async function copyOut() {
  if (!output.value) { message.warning('没有可复制的内容'); return }
  try { await navigator.clipboard.writeText(output.value); message.success('已复制') }
  catch { message.warning('复制失败，请手动选择') }
}
</script>

<template>
  <a-row :gutter="12">
    <a-col :xs="24" :lg="8">
      <a-card :bordered="false" title="服务器插件（plugins/）" size="small">
        <div class="toolbar">
          <a-input v-model:value="filter" placeholder="筛选插件名" allow-clear style="width: 180px" />
          <a-button @click="load">刷新列表</a-button>
        </div>
        <a-table :data-source="filtered" :loading="loading" row-key="name" size="small" :pagination="false">
          <a-table-column title="名称" data-index="name" />
          <a-table-column title="操作" width="80">
            <template #default="{ record }"><a @click="run(record.name)">执行</a></template>
          </a-table-column>
        </a-table>
      </a-card>
    </a-col>
    <a-col :xs="24" :lg="16">
      <a-card :bordered="false" size="small">
        <template #title>输出</template>
        <template #extra>
          <a-space>
            <a-button size="small" @click="copyOut">复制</a-button>
            <a-button size="small" @click="output = ''">清空</a-button>
          </a-space>
        </template>
        <pre class="out">{{ output || '（无输出）' }}</pre>
      </a-card>
    </a-col>
  </a-row>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 8px; }
.out { min-height: 260px; margin: 0; white-space: pre-wrap; font-family: ui-monospace, monospace; font-size: 12px; }
</style>
