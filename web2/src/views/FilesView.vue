<script setup lang="ts">
// 文件管理：列表 + 进入目录 + 编辑 + 上传 + 下载 + 重命名 + 新建目录 + 删除
import { computed, h, onMounted, ref } from 'vue'
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

// 文件图标：用 material-icon-theme 集合（Material Icons 浏览器扩展同源图标集）
const EXT_ICON: Record<string, string> = {
  ts: 'typescript', tsx: 'react', js: 'javascript', jsx: 'react', mjs: 'javascript', cjs: 'javascript',
  vue: 'vue', json: 'json', md: 'markdown', markdown: 'markdown', txt: 'document', log: 'log',
  py: 'python', go: 'go', rs: 'rust', java: 'java', kt: 'kotlin', swift: 'swift', rb: 'ruby', php: 'php',
  c: 'c', h: 'c', cpp: 'cpp', hpp: 'cpp', sh: 'console', bash: 'console', zsh: 'console',
  html: 'html', htm: 'html', css: 'css', scss: 'css', less: 'css', yml: 'yaml', yaml: 'yaml',
  toml: 'toml', ini: 'settings', conf: 'settings', env: 'settings', sql: 'database', db: 'database',
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', bmp: 'image', webp: 'image', svg: 'svg',
  ico: 'image', pdf: 'pdf', zip: 'zip', gz: 'zip', tar: 'zip', xz: 'zip', rar: 'zip',
  doc: 'word', docx: 'word', xls: 'document', xlsx: 'document', ppt: 'powerpoint', pptx: 'powerpoint',
  mp3: 'audio', wav: 'audio', flac: 'audio', mp4: 'video', mkv: 'video', avi: 'video', mov: 'video',
  lock: 'lock', pem: 'key', key: 'key', crt: 'certificate', cer: 'certificate',
}
// 特例文件名（Dockerfile / Makefile / .gitignore 等）单独映射
const NAME_ICON: Record<string, string> = {
  dockerfile: 'docker', 'docker-compose.yml': 'docker', 'docker-compose.yaml': 'docker',
  makefile: 'makefile', 'cmakelists.txt': 'cmake',
  '.gitignore': 'git', '.gitattributes': 'git', '.gitmodules': 'git',
  'package.json': 'npm', 'package-lock.json': 'npm', 'pnpm-lock.yaml': 'npm', 'yarn.lock': 'npm',
  'tsconfig.json': 'tsconfig', 'vite.config.ts': 'vite', 'vite.config.js': 'vite',
  'webpack.config.js': 'webpack', '.babelrc': 'babel', '.eslintrc': 'eslint',
  '.eslintrc.js': 'eslint', '.eslintrc.json': 'eslint', '.prettierrc': 'prettier',
  '.editorconfig': 'editorconfig', 'go.mod': 'go', 'go.sum': 'go',
  'cargo.toml': 'rust', 'cargo.lock': 'rust', 'requirements.txt': 'python', 'pyproject.toml': 'python',
  'readme.md': 'readme', readme: 'readme', license: 'license', 'license.md': 'license',
  changelog: 'changelog', 'changelog.md': 'changelog',
  'nginx.conf': 'nginx', 'jenkinsfile': 'jenkins', '.gitlab-ci.yml': 'gitlab',
  'pom.xml': 'maven', 'build.gradle': 'gradle', 'terraform.tf': 'terraform', 'chart.yaml': 'helm',
}

// 目录专属图标（按目录名），material-icon-theme 里存在的 folder-* 图标
const FOLDER_ICON: Record<string, string> = {
  src: 'folder-src', '.git': 'folder-git', dist: 'folder-dist', test: 'folder-test',
  docs: 'folder-docs', config: 'folder-config', api: 'folder-api', lib: 'folder-lib',
  public: 'folder-public', scripts: 'folder-scripts', home: 'folder-home', log: 'folder-log',
  backup: 'folder-backup', database: 'folder-database', docker: 'folder-docker',
  kubernetes: 'folder-kubernetes', vue: 'folder-vue', angular: 'folder-angular',
  python: 'folder-python', go: 'folder-go', rust: 'folder-rust', java: 'folder-java',
  theme: 'folder-theme', images: 'folder-images', font: 'folder-font',
  download: 'folder-download', desktop: 'folder-desktop', video: 'folder-video',
  trashes: 'folder-trash',
}

const DEFAULT_FILE_ICON = 'document'

function fileIcon(r: any) {
  if (isDir(r)) {
    const folder = FOLDER_ICON[nameOf(r).toLowerCase()]
    return `material-icon-theme:${folder || 'folder-base'}`
  }
  const name = nameOf(r)
  const lower = name.toLowerCase()
  // 先按特例文件名匹配，再按扩展名
  const byName = NAME_ICON[lower]
  if (byName) return `material-icon-theme:${byName}`
  const ext = lower.includes('.') ? lower.split('.').pop()! : ''
  return `material-icon-theme:${EXT_ICON[ext] || DEFAULT_FILE_ICON}`
}

// ---- 左侧文件树（自实现：懒加载 + 点击进入）----
const treeLoading = ref(false)
const dirs = ref<Record<string, string[]>>({})   // path -> 子目录名
const openDirs = ref<Record<string, boolean>>({ '/': true })

function joinPath(parent: string, name: string) {
  return (parent.endsWith('/') ? parent : parent + '/') + name
}

async function fetchDirs(path: string) {
  const res: any = await post('/file/ls', { id, path })
  const items: any[] = res?.result?.items || []
  dirs.value[path] = items.filter((i) => isDir(i)).map((i) => nameOf(i))
  dirs.value = { ...dirs.value }
}

async function expandTree(path: string) {
  if (!dirs.value[path]) await fetchDirs(path)
  openDirs.value[path] = !openDirs.value[path]
  openDirs.value = { ...openDirs.value }
}

async function loadTree() {
  treeLoading.value = true
  try { await fetchDirs('/') } finally { treeLoading.value = false }
}

// 拍平成一维列表渲染（depth 控制缩进），避免递归组件
type FlatNode = { path: string; name: string; depth: number; hasChildren: boolean; open: boolean }
const flatTree = computed<FlatNode[]>(() => {
  const out: FlatNode[] = []
  const walk = (path: string, depth: number) => {
    const list = dirs.value[path] || []
    for (const name of list) {
      const full = joinPath(path, name)
      const open = !!openDirs.value[full]
      out.push({ path: full, name, depth, hasChildren: true, open })
      if (open && dirs.value[full]) walk(full, depth + 1)
    }
  }
  walk('/', 0)
  return out
})

function pickDir(node: FlatNode) { load(node.path) }

const nameOf = (r: any) => r.name ?? r.Name ?? ''
const isDir = (r: any) => !!(r.isDir ?? r.IsDir)
const base = () => (path.value.endsWith('/') ? path.value : path.value + '/')
const full = (r: any) => base() + nameOf(r)

async function load(p = path.value) {
  loading.value = true
  path.value = p            // 先更新路径：即便列目录失败，界面也应停在目标目录
  try {
    const res: any = await post('/file/ls', { id, path: p })
    rows.value = res?.result?.items || []
  } catch {
    rows.value = []
  } finally {
    loading.value = false
  }
}
onMounted(() => { load('/'); loadTree() })

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

    <a-row :gutter="12">
      <a-col :xs="24" :lg="6">
        <a-card size="small" title="文件树" :loading="treeLoading" :body-style="{ padding: '8px', maxHeight: 'calc(100vh - 280px)', overflow: 'auto' }">
          <div class="tree-root" @click="load('/')">/</div>
          <div
            v-for="n in flatTree" :key="n.path" class="tree-row"
            :style="{ paddingLeft: 12 + n.depth * 12 + 'px' }"
          >
            <span class="tw" @click.stop="expandTree(n.path)">{{ n.open ? '▾' : '▸' }}</span>
            <span class="tn" :class="{ active: path === n.path }" @click.stop="pickDir(n)">{{ n.name }}</span>
          </div>
          <div v-if="!flatTree.length" class="hint">（无子目录）</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="18">
    <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="name" size="small" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'name'">
          <a @dblclick="openFile(record)" @click="enter(record)">
            <span class="file-ico" :class="`i-${fileIcon(record)}`" />
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
      </a-col>
    </a-row>

    <a-modal v-model:open="editor" :title="editPath" width="720px" @ok="saveFile">
      <a-textarea v-model:value="editContent" :rows="18" class="mono" />
    </a-modal>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; flex-wrap: wrap; }
.hint { color: rgba(0, 0, 0, 0.45); font-size: 12px; }
.tree-root { cursor: pointer; font-family: ui-monospace, monospace; font-size: 12px; padding: 2px 0; }
.tree-row { display: flex; align-items: center; gap: 4px; font-size: 12px; line-height: 20px; }
.tw { width: 12px; color: rgba(0,0,0,.45); cursor: pointer; user-select: none; }
.tn { cursor: pointer; font-family: ui-monospace, monospace; }
.tn.active { color: #1677ff; }
.danger { color: #ff4d4f; }
.file-ico { width: 16px; height: 16px; display: inline-block; vertical-align: -3px; margin-right: 6px; }
</style>
