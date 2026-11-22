<script setup lang="ts">
// 文件管理：列表 + 进入目录 + 编辑 + 上传 + 下载 + 重命名 + 新建目录 + 删除
import { h, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { downloadURL, post } from '../api/http'

const route = useRoute()
const id = Number(route.params.id)
const path = ref('/')
const rows = ref<any[]>([])
const loading = ref(false)
const editor = ref(false)
const editPath = ref('')
const editContent = ref('')
const uploadRef = ref<HTMLInputElement>()

const columns = [
  { title: '文件名', dataIndex: 'name', width: 300 },
  { title: '大小', dataIndex: 'size', width: 110 },
  { title: '修改时间', dataIndex: 'time', width: 220 },
  { title: '操作', key: 'action', width: 240 },
]

const nameOf = (r: any) => r.name ?? r.Name ?? ''
const isDir = (r: any) => !!(r.isDir ?? r.IsDir)
const base = () => (path.value.endsWith('/') ? path.value : path.value + '/')
const full = (r: any) => base() + nameOf(r)

async function load(p = path.value) {
  loading.value = true
  try {
    const res: any = await post('/file/ls', { id, path: p })
    rows.value = res?.result?.items || []
    path.value = p
  } finally {
    loading.value = false
  }
}
onMounted(() => load('/'))

function enter(r: any) { if (isDir(r)) load(full(r)) }
function up() {
  const parts = path.value.split('/').filter(Boolean)
  parts.pop()
  load('/' + parts.join('/'))
}

async function openFile(r: any) {
  if (isDir(r)) return enter(r)
  const res: any = await post('/file/cat', { id, path: base(), target: nameOf(r) })
  editPath.value = full(r)
  editContent.value = res?.result?.content || ''
  editor.value = true
}

async function saveFile() {
  await post('/file/edit', { id, path: editPath.value, content: editContent.value })
  message.success('已保存')
  editor.value = false
  load()
}

function mkdir() {
  let name = ''
  Modal.confirm({
    title: '新建文件夹',
    content: () => h('input', {
      class: 'ant-input', style: 'width:100%',
      onInput: (e: Event) => { name = (e.target as HTMLInputElement).value },
    }),
    onOk: async () => {
      if (!name) return
      await post('/file/mkdir', { id, path: base() + name })
      message.success('已创建'); load()
    },
  })
}

function rename(r: any) {
  let name = nameOf(r)
  Modal.confirm({
    title: '重命名',
    content: () => h('input', {
      class: 'ant-input', style: 'width:100%', value: name,
      onInput: (e: Event) => { name = (e.target as HTMLInputElement).value },
    }),
    onOk: async () => {
      if (!name) return
      await post('/file/mv', { id, path: `${full(r)}|<-${base() + name}` })
      message.success('已重命名'); load()
    },
  })
}

function remove(r: any) {
  Modal.confirm({
    title: '是否确认删除？',
    onOk: async () => { await post('/file/rm', { id, path: full(r) }); message.success('已删除'); load() },
  })
}

function download(r: any) {
  if (isDir(r)) return
  location.href = downloadURL('/file/downloadtobrowser', { id, path: base(), target: nameOf(r) })
}

async function onUpload(e: Event) {
  const files = (e.target as HTMLInputElement).files
  if (!files?.length) return
  const fd = new FormData()
  fd.append('id', String(id))
  fd.append('path', path.value)
  fd.append('file', files[0])
  const token = localStorage.getItem('vshell.token') || ''
  const resp = await fetch('/api/file/upload', { method: 'POST', body: fd, headers: { Token: token } })
  const data = await resp.json()
  if (data.code === 0) { message.success('上传成功'); load() } else message.error(data.message || '上传失败')
  ;(e.target as HTMLInputElement).value = ''
}
</script>

<template>
  <a-card :bordered="false" :body-style="{ padding: '12px' }">
    <div class="toolbar">
      <a-input v-model:value="path" style="width: 320px" class="mono" @press-enter="load(path)" />
      <a-button @click="load(path)">打开</a-button>
      <a-button @click="up">上级</a-button>
      <a-button @click="mkdir">新建文件夹</a-button>
      <a-button @click="uploadRef?.click()">上传插件</a-button>
      <input ref="uploadRef" type="file" style="display:none" @change="onUpload" />
      <span class="hint mono">客户端 #{{ id }}</span>
    </div>

    <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="name" size="middle" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'name'">
          <a @dblclick="openFile(record)" @click="enter(record)">
            {{ nameOf(record) }}{{ isDir(record) ? '/' : '' }}
          </a>
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a @click="openFile(record)">{{ isDir(record) ? '进入' : '编辑' }}</a>
            <a v-if="!isDir(record)" @click="download(record)">下载</a>
            <a @click="rename(record)">重命名</a>
            <a class="danger" @click="remove(record)">删除文件</a>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="editor" :title="editPath" width="720px" @ok="saveFile">
      <a-textarea v-model:value="editContent" :rows="18" class="mono" />
    </a-modal>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.hint { color: rgba(0, 0, 0, 0.45); font-size: 12px; }
.danger { color: #ff4d4f; }
</style>
