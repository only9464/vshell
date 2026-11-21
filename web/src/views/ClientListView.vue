<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { clientApi, type ClientItem } from '../api'

const router = useRouter()
const rows = ref<ClientItem[]>([])
const total = ref(0)
const online = ref(0)
const search = ref('')
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await clientApi.list(1, 100, search.value)
    rows.value = res.items || []
    total.value = res.total
    online.value = res.clientOnlineCount
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function editRemark(row: ClientItem) {
  const { value } = await ElMessageBox.prompt('备注', '修改备注', { inputValue: row.Remark })
  await clientApi.editRemark(row.Id, value || '')
  ElMessage.success('已保存')
  load()
}

// 详情抽屉：展示面板拿到的全部字段（含原版 14 个列契约字段）
const drawer = ref(false)
const current = ref<ClientItem | null>(null)
function detail(row: ClientItem) {
  current.value = row
  drawer.value = true
}
const detailFields: { key: keyof ClientItem; label: string }[] = [
  { key: 'Id', label: 'ID' },
  { key: 'VerifyKey', label: 'Vkey' },
  { key: 'Tp', label: '通道' },
  { key: 'Addr', label: '外网地址' },
  { key: 'LocalIP', label: '内网 IP' },
  { key: 'HostName', label: '主机名' },
  { key: 'UserName', label: '用户' },
  { key: 'OsName', label: '系统' },
  { key: 'Arch', label: '架构' },
  { key: 'ProcessName', label: '进程' },
  { key: 'Version', label: '版本' },
  { key: 'Location', label: '归属地' },
  { key: 'Remark', label: '备注' },
] as const
function fmtTime(v: unknown) {
  if (!v) return '-'
  const n = Number(v)
  if (!n) return String(v)
  return new Date(n * 1000).toLocaleString()
}

async function remove(row: ClientItem) {
  await ElMessageBox.confirm(`删除客户端 ${row.HostName || row.Id}？`, '确认', { type: 'warning' })
  await clientApi.delList([row.Id])
  ElMessage.success('已删除')
  load()
}
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-input v-model="search" placeholder="搜索主机/用户/备注" style="width: 240px"
                clearable @keyup.enter="load" />
      <el-button @click="load">刷新</el-button>
      <span class="mono">共 {{ total }} 台，在线 {{ online }}</span>
    </div>
    <el-table :data="rows" border size="small">
      <el-table-column prop="Id" label="ID" width="60" />
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.IsConnect ? 'success' : 'info'" size="small">
            {{ row.IsConnect ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="主机名" min-width="120">
        <template #default="{ row }">
          <el-button link type="primary" @click="detail(row)">{{ row.HostName || '(未知)' }}</el-button>
        </template>
      </el-table-column>
      <el-table-column prop="UserName" label="用户" width="110" />
      <el-table-column prop="OsName" label="系统" width="110" />
      <el-table-column prop="Tp" label="通道" width="90" />
      <el-table-column prop="Addr" label="外网地址" min-width="140" />
      <el-table-column prop="LocalIP" label="内网 IP" width="120" />
      <el-table-column prop="ProcessName" label="进程" width="110" />
      <el-table-column prop="Remark" label="备注" min-width="100" />
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="router.push(`/terminal/${row.Id}`)">终端</el-button>
          <el-button link type="primary" @click="router.push(`/files/${row.Id}`)">文件</el-button>
          <el-button link type="primary" @click="router.push(`/screen/${row.Id}`)">屏幕</el-button>
          <el-button link type="primary" @click="router.push(`/screenshot/${row.Id}`)">截图</el-button>
          <el-button link type="primary" @click="router.push(`/plugins/${row.Id}`)">插件</el-button>
          <el-button link @click="editRemark(row)">备注</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-drawer v-model="drawer" title="客户端详情" size="420px">
      <template v-if="current">
        <el-descriptions :column="1" border size="small">
          <el-descriptions-item v-for="f in detailFields" :key="f.key" :label="f.label">
            <span class="mono">{{ (current as any)[f.key] || '-' }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="在线">
            <el-tag :type="current.IsConnect ? 'success' : 'info'" size="small">
              {{ current.IsConnect ? '在线' : '离线' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="最后心跳">{{ fmtTime((current as any).PingCheckTime) }}</el-descriptions-item>
        </el-descriptions>
        <div class="drawer-actions">
          <el-button type="primary" @click="router.push(`/terminal/${current.Id}`)">终端</el-button>
          <el-button @click="router.push(`/files/${current.Id}`)">文件</el-button>
          <el-button @click="router.push(`/screen/${current.Id}`)">屏幕</el-button>
          <el-button @click="router.push(`/screenshot/${current.Id}`)">截图</el-button>
          <el-button @click="router.push(`/plugins/${current.Id}`)">插件</el-button>
        </div>
      </template>
    </el-drawer>
  </div>
</template>

<style scoped>
.drawer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}
</style>
