<script setup lang="ts">
// 监听器：列表 + 新建/编辑 + 启停/删除（对齐 4.9.3 的字段与操作）
import { onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { post } from '../api/http'

const loading = ref(false)
const rows = ref<any[]>([])
const total = ref(0)
const open = ref(false)
const editing = ref<number | null>(null)
const form = reactive({ Mode: 'http', Vkey: '', Host: '127.0.0.1', Port: 8443, PingInterval: 1, Remark: '', EncryptSalt: '' })

const modes = ['http', 'https', 'kcp', 'websocket', 'dns', 'cdn_websocket']
const columns = [
  { title: 'ID', dataIndex: 'Id', width: 70 },
  { title: '模式', dataIndex: 'Mode', width: 120 },
  { title: '状态', dataIndex: 'Status', width: 90 },
  { title: '监听地址', dataIndex: 'ListenAddr', width: 180 },
  { title: 'Vkey', dataIndex: 'Vkey', width: 140 },
  { title: '加密盐', dataIndex: 'EncryptSalt', width: 140 },
  { title: '备注', dataIndex: 'Remark' },
  { title: '操作', key: 'action', width: 200, fixed: 'right' },
]

async function load() {
  loading.value = true
  try {
    const res: any = await post('/listener/list', { page: 1, pageSize: 100 })
    rows.value = res?.result?.items || []
    total.value = res?.result?.total || 0
  } finally {
    loading.value = false
  }
}
onMounted(load)

function create() {
  editing.value = null
  Object.assign(form, { Mode: 'http', Vkey: '', Host: '127.0.0.1', Port: 8443, PingInterval: 1, Remark: '', EncryptSalt: '' })
  open.value = true
}

function edit(row: any) {
  editing.value = row.Id
  Object.assign(form, {
    Mode: row.Mode, Vkey: row.Vkey, Host: '127.0.0.1', Port: row.Port || 8443,
    PingInterval: 1, Remark: row.Remark || '', EncryptSalt: row.EncryptSalt || '',
  })
  open.value = true
}

async function submit() {
  if (!form.Vkey) { message.warning('Vkey 必填'); return }
  if (editing.value) {
    await post('/listener/edit', { Id: editing.value, ...form })
    message.success('已保存')
  } else {
    await post('/listener/add', { ...form })
    message.success('已创建')
  }
  open.value = false
  load()
}

async function toggle(row: any) {
  await post(row.Status ? '/listener/stop' : '/listener/start', { id: row.Id })
  load()
}

function remove(row: any) {
  Modal.confirm({
    title: `删除监听器 ${row.Id}？`,
    onOk: async () => { await post('/listener/del', { id: row.Id }); message.success('已删除'); load() },
  })
}
</script>

<template>
  <a-card :bordered="false">
    <div class="toolbar">
      <a-button type="primary" @click="create">新建监听器</a-button>
      <a-button @click="load">刷新</a-button>
      <span class="hint">共 {{ total }} 个</span>
    </div>

    <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="Id" size="middle" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'Status'">
          <a-tag :color="record.Status ? 'green' : 'default'">{{ record.Status ? '运行' : '停止' }}</a-tag>
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a @click="toggle(record)">{{ record.Status ? '停止' : '启动' }}</a>
            <a @click="edit(record)">编辑</a>
            <a class="danger" @click="remove(record)">删除</a>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="open" :title="editing ? '编辑监听器' : '新建监听器'" :width="520" @ok="submit">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="模式">
          <a-select v-model:value="form.Mode">
            <a-select-option v-for="m in modes" :key="m" :value="m">{{ m }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="Vkey"><a-input v-model:value="form.Vkey" /></a-form-item>
        <a-form-item label="监听地址"><a-input v-model:value="form.Host" /></a-form-item>
        <a-form-item label="端口"><a-input-number v-model:value="form.Port" :min="1" :max="65535" style="width: 100%" /></a-form-item>
        <a-form-item label="心跳(秒)"><a-input-number v-model:value="form.PingInterval" :min="1" style="width: 100%" /></a-form-item>
        <a-form-item label="加密盐"><a-input v-model:value="form.EncryptSalt" /></a-form-item>
        <a-form-item label="备注"><a-input v-model:value="form.Remark" /></a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.hint { color: rgba(0, 0, 0, 0.45); }
.danger { color: #ff4d4f; }
</style>
