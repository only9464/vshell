<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { fileApi } from '../api'


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
  if (!row.IsDir) return
  const base = path.value.endsWith('/') ? path.value : path.value + '/'
  load(base + row.Name)
}
function up() {
  const parts = path.value.split('/').filter(Boolean)
  parts.pop()
  load('/' + parts.join('/'))
}
async function mkdir() {
  const name = prompt('目录名')
  if (!name) return
  await fileApi.mkdir(id, path.value, name)
  ElMessage.success('已创建')
  load()
}
async function remove(row: any) {
  await fileApi.rm(id, (path.value.endsWith('/') ? path.value : path.value + '/') + row.Name)
  ElMessage.success('已删除')
  load()
}
const editor = ref(false)
const editorPath = ref('')
const editorContent = ref('')

async function openFile(row: any) {
  if (row.IsDir) return enter(row)
  const full = (path.value.endsWith('/') ? path.value : path.value + '/') + row.Name
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
      <label class="upload-btn">
        <input type="file" style="display:none" @change="upload" />
        <el-button>上传</el-button>
      </label>
      <span class="mono">客户端 #{{ id }}</span>
    </div>
    <el-table :data="rows" border size="small" @row-dblclick="openFile">
      <el-table-column prop="Name" label="名称" min-width="240">
        <template #default="{ row }">
          <span :class="{ dir: row.IsDir }">{{ row.Name }}{{ row.IsDir ? '/' : '' }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="Size" label="大小" width="120" />
      <el-table-column prop="Time" label="修改时间" width="220" />
      <el-table-column label="操作" width="160">
        <template #default="{ row }">
          <el-button link type="primary" @click="openFile(row)">{{ row.IsDir ? '进入' : '编辑' }}</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
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
