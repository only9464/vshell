<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { listenerApi, type ListenerItem } from '../api'

const rows = ref<ListenerItem[]>([])
const total = ref(0)
const loading = ref(false)
const dialog = ref(false)
const form = reactive({
  Mode: 'http', Vkey: '', Remark: '', Host: '127.0.0.1', Port: 8443, PingInterval: 1,
})

const modes = ['http', 'https', 'kcp', 'websocket', 'dns', 'cdn_websocket']

async function load() {
  loading.value = true
  try {
    const res = await listenerApi.list()
    rows.value = res.items || []
    total.value = res.total
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function add() {
  if (!form.Vkey) return ElMessage.warning('Vkey 必填')
  await listenerApi.add({ ...form })
  dialog.value = false
  ElMessage.success('已创建（会自动启动）')
  load()
}

async function toggle(row: ListenerItem) {
  try {
    if (row.Status) await listenerApi.stop(row.Id)
    else await listenerApi.start(row.Id)
    ElMessage.success('已切换')
  } finally {
    load()
  }
}

async function remove(row: ListenerItem) {
  await ElMessageBox.confirm(`删除监听器 ${row.Id}？`, '确认', { type: 'warning' })
  await listenerApi.del(row.Id)
  ElMessage.success('已删除')
  load()
}
</script>

<template>
  <div class="page" v-loading="loading">
    <div class="toolbar">
      <el-button type="primary" @click="dialog = true">新建监听器</el-button>
      <el-button @click="load">刷新</el-button>
      <span class="mono">共 {{ total }} 个</span>
    </div>
    <el-table :data="rows" border size="small">
      <el-table-column prop="Id" label="ID" width="60" />
      <el-table-column prop="Mode" label="模式" width="120" />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="row.Status ? 'success' : 'info'" size="small">{{ row.Status ? '运行' : '停止' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="ListenAddr" label="监听地址" min-width="150" />
      <el-table-column prop="Vkey" label="Vkey" min-width="120" />
      <el-table-column prop="EncryptSalt" label="加密盐" min-width="120" />
      <el-table-column prop="Remark" label="备注" min-width="100" />
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" @click="toggle(row)">{{ row.Status ? '停止' : '启动' }}</el-button>
          <el-button link type="danger" @click="remove(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialog" title="新建监听器" width="460px">
      <el-form label-width="90px">
        <el-form-item label="模式">
          <el-select v-model="form.Mode" style="width: 100%">
            <el-option v-for="m in modes" :key="m" :label="m" :value="m" />
          </el-select>
        </el-form-item>
        <el-form-item label="Vkey"><el-input v-model="form.Vkey" /></el-form-item>
        <el-form-item label="监听地址"><el-input v-model="form.Host" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="form.Port" :min="1" :max="65535" /></el-form-item>
        <el-form-item label="心跳(秒)"><el-input-number v-model="form.PingInterval" :min="1" /></el-form-item>
        <el-form-item label="备注"><el-input v-model="form.Remark" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialog = false">取消</el-button>
        <el-button type="primary" @click="add">创建</el-button>
      </template>
    </el-dialog>
  </div>
</template>
