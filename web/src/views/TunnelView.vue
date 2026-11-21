<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { clientApi, post, type ClientItem } from '../api'

interface TunnelItem {
  Id: number
  ClientId?: number
  ClientID?: number
  Port: number
  Mode: string
  Target: string
  Remark: string
}

const rows = ref<TunnelItem[]>([])
const clients = ref<ClientItem[]>([])
const loading = ref(false)
const dialog = ref(false)
const form = reactive({ clientId: 0, Port: 18888, Mode: 'tcp', Target: '127.0.0.1:22', Remark: '' })

async function load() {
  loading.value = true
  try {
    const res = await post<{ items: TunnelItem[] | null; total: number }>('/tunnel/list', { page: 1, pageSize: 100 })
    rows.value = res.items || []
    const cl = await clientApi.list()
    clients.value = cl.items || []
    if (!form.clientId && clients.value.length) form.clientId = clients.value[0].Id
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function add() {
  if (!form.clientId) return ElMessage.warning('先选择客户端')
  await post('/tunnel/add', { ...form })
  dialog.value = false
  ElMessage.success('已创建')
  load()
}

async function start(row: TunnelItem) {
  const serverAddr = prompt('agent 拨号的隧道地址（通常是服务器 IP:端口）', `127.0.0.1:${row.Port}`)
  if (!serverAddr) return
  await post('/tunnel/start', { id: row.Id, serverAddr })
  ElMessage.success('已启动')
}

async function stop(row: TunnelItem) {
  await post('/tunnel/stop', { id: row.Id })
  ElMessage.success('已停止')
}

async function remove(row: TunnelItem) {
  await ElMessageBox.confirm(`删除隧道 ${row.Id}？`, '确认', { type: 'warning' })
  await post('/tunnel/del', { id: row.Id })
  ElMessage.success('已删除')
  load()
}
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-button type="primary" @click="dialog = true">新建隧道</el-button>
      <el-button @click="load">刷新</el-button>
      <span class="mono">共 {{ rows.length }} 条</span>
    </div>
    <el-table :data="rows" border size="small">
      <el-table-column prop="Id" label="ID" width="60" />
      <el-table-column prop="Port" label="服务器端口" width="120" />
      <el-table-column prop="Mode" label="模式" width="90" />
      <el-table-column prop="Target" label="目标" min-width="180" />
      <el-table-column prop="Remark" label="备注" min-width="120" />
      <el-table-column label="操作" width="240" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="start(row)">启动</el-button>
          <el-button link @click="stop(row)">停止</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
          <template #empty>
        <el-empty :description="loading ? '加载中…' : '还没有隧道'" />
      </template>
    </el-table>

    <el-dialog v-model="dialog" title="新建隧道" width="480px">
      <el-form label-width="110px">
        <el-form-item label="客户端">
          <el-select v-model="form.clientId" style="width: 100%">
            <el-option v-for="c in clients" :key="c.Id" :label="`#${c.Id} ${c.HostName || c.Addr}`" :value="c.Id" />
          </el-select>
        </el-form-item>
        <el-form-item label="服务器端口"><el-input-number v-model="form.Port" :min="1" :max="65535" /></el-form-item>
        <el-form-item label="模式">
          <el-select v-model="form.Mode" style="width: 120px">
            <el-option label="tcp" value="tcp" />
          </el-select>
        </el-form-item>
        <el-form-item label="目标地址"><el-input v-model="form.Target" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.Remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="add">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>
