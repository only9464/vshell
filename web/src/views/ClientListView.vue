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
      <el-table-column prop="HostName" label="主机名" min-width="120" />
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
  </div>
</template>
