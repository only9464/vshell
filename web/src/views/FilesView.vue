<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { fileApi } from '../api'
import { downloadURL } from '../api/http'


const route = useRoute()
const id = Number(route.params.id)
const path = ref('/')
const rows = ref<any[]>([])
const loading = ref(false)

async function load(p = path.value) {
  loading.value = true
  try {
    const res = await fileApi.ls(id, p)
    rows.value = res.items || []
    path.value = p
  } finally {
    loading.value = false
  }
}
onMounted(() => load('/'))

function enter(row: any) {
  if (!isDir(row)) return
  const base = path.value.endsWith('/') ? path.value : path.value + '/'
  load(base + nameOf(row))
}
function up() {
  const parts = path.value.split('/').filter(Boolean)
  parts.pop()
  load('/' + parts.join('/'))
}
async function mkdir() {
  // 用 Element Plus 的输入框而不是原生 prompt：原生 prompt 在无头浏览器里
  // 直接返回 null，既影响自动化验证，体验也差。
  const { value } = await ElMessageBox.prompt('目录名', '新建目录', { inputValue: '' })
  if (!value) return
  await fileApi.mkdir(id, path.value, value)
  ElMessage.success('已创建')
  load()
}
function fullPath(row: any) {
  const base = path.value.endsWith('/') ? path.value : path.value + '/'
  return base + nameOf(row)
}

async function remove(row: any) {
  await fileApi.rm(id, fullPath(row))
  ElMessage.success('已删除')
  load()
}

// 浏览器下载：走 /file/downloadtobrowser（服务端回附件，带 token 直链）
function download(row: any) {
  if (isDir(row)) return
  const base = path.value.endsWith('/') ? path.value : path.value + '/'
  window.open(downloadURL('/file/downloadtobrowser', { id, path: base, target: nameOf(row) }), '_blank')
}

// 重命名：后端 mv 用 "src|<-dst" 约定
async function rename(row: any) {
  const old = nameOf(row)
  const { value: name } = await ElMessageBox.prompt('新名称', '重命名', { inputValue: old })
  if (!name || name === old) return
  const base = path.value.endsWith('/') ? path.value : path.value + '/'
  await fileApi.mv(id, base + old, base + name)
  ElMessage.success('已重命名')
  load()
}
// 后端 /file/ls 返回小写键（name/size/time/isDir）；这里做一次兼容读取，
// 避免字段大小写不一致时表格整列空白（R45 实测）。
type FsEntry = { name?: string; Name?: string; isDir?: boolean; IsDir?: boolean }
const nameOf = (row: FsEntry) => row.name ?? row.Name ?? ''
const isDir = (row: FsEntry) => !!(row.isDir ?? row.IsDir)

const editor = ref(false)
const editorPath = ref('')
const editorContent = ref('')

async function openFile(row: any) {
  if (isDir(row)) return enter(row)
  const full = (path.value.endsWith('/') ? path.value : path.value + '/') + nameOf(row)
  const res: any = await fileApi.cat(id, full)
  editorPath.value = full
  editorContent.value = res?.content ?? String(res ?? '')
  editor.value = true
}

async function saveFile() {
  const idx = editorPath.value.lastIndexOf('/')
  const dir = idx > 0 ? editorPath.value.slice(0, idx) : '/'
  const name = editorPath.value.slice(idx + 1)
  await fileApi.edit(id, dir, name, editorContent.value)
  ElMessage.success('已保存')
  editor.value = false
  load(path.value)
}

async function wget() {
  const url = prompt('要下载到目标的 URL')
  if (!url) return
  await fileApi.wget(id, path.value, url)
  ElMessage.success('已下发 wget 任务')
}
async function upload(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  await fileApi.upload(id, path.value, file)
  ElMessage.success('已上传并下发到目标')
  input.value = ''
  load()
}
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-button @click="up">上级</el-button>
      <el-input v-model="path" style="width: 380px" @keyup.enter="load(path)" />
      <el-button @click="load(path)">打开</el-button>
      <el-button @click="mkdir">新建目录</el-button>
      <el-button @click="wget">wget</el-button>
      <label class="upload-btn">
        <input type="file" style="display:none" @change="upload" />
        <el-button>上传</el-button>
      </label>
      <span class="mono">客户端 #{{ id }}</span>
    </div>
    <el-table :data="rows" border size="small" @row-dblclick="openFile">
      <el-table-column prop="name" label="名称" min-width="240">
        <template #default="{ row }">
          <span :class="{ dir: isDir(row) }">{{ nameOf(row) }}{{ isDir(row) ? '/' : '' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="size" label="大小" width="120" />
      <el-table-column prop="time" label="修改时间" width="220" />
      <el-table-column label="操作" width="240">
        <template #default="{ row }">
          <el-button link type="primary" @click="openFile(row)">{{ isDir(row) ? '进入' : '编辑' }}</el-button>
          <el-button v-if="!isDir(row)" link type="primary" @click="download(row)">下载</el-button>
          <el-button link @click="rename(row)">重命名</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
          <template #empty>
        <el-empty :description="loading ? '加载中…' : '目录为空'" />
      </template>
    </el-table>
    <el-dialog v-model="editor" :title="editorPath" width="70%">
      <el-input v-model="editorContent" type="textarea" :rows="18" class="mono" />
      <template #footer>
        <el-button @click="editor = false">取消</el-button>
        <el-button type="primary" @click="saveFile">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.dir { color: #2563eb; cursor: pointer; }
</style>
