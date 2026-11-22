<script setup lang="ts">
// 监听列表：列与表单字段按 4.9.3 的 bundle 原文（vD6-3nvsI.js）
// 列：ID / 备注 / 模式 / 监听地址 / 外网连接地址 / 超时心跳次数（次）/ 心跳间隔（秒）/ 状态
import { onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { post } from '../api/http'

const loading = ref(false)
const rows = ref<any[]>([])
const total = ref(0)
const open = ref(false)
const formRef = ref<any>(null)
const editing = ref<number | null>(null)
const checkedKeys = ref<number[]>([])

const form = reactive({
  Id: 0, Mode: 'http', Remark: '', Host: '127.0.0.1', Port: 8443,
  ConnectAddr: '', WsConnectAddr: '', DNSDomain: '',
  DisconnectTimeout: 60, PingInterval: 1, Vkey: '', EncryptSalt: '',
})

// 4.9.3 的监听方式（其表单里 tp 是 RadioButtonGroup）
const modes = ['http', 'https', 'kcp', 'websocket', 'dns', 'cdn_websocket']

const columns = [
  { title: 'ID', dataIndex: 'Id', width: 70, sorter: true },
  { title: '备注', dataIndex: 'Remark', width: 120 },
  { title: '模式', dataIndex: 'Mode', width: 130 },
  { title: '监听地址', dataIndex: 'ListenAddr', width: 170 },
  { title: '外网连接地址', dataIndex: 'ConnectAddr', width: 170 },
  { title: '超时心跳次数（次）', dataIndex: 'DisconnectTimeout', width: 150 },
  { title: '心跳间隔（秒）', dataIndex: 'PingInterval', width: 130 },
  { title: '状态', dataIndex: 'Status', width: 90 },
  { title: '操作', key: 'action', width: 180, fixed: 'right' },
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
  Object.assign(form, {
    Id: 0, Mode: 'http', Remark: '', Host: '127.0.0.1', Port: 8443,
    ConnectAddr: '', WsConnectAddr: '', DNSDomain: '',
    DisconnectTimeout: 60, PingInterval: 1, Vkey: '', EncryptSalt: '',
  })
  open.value = true
}

function edit(row: any) {
  editing.value = row.Id
  const [host, port] = String(row.ListenAddr || '').split(':')
  Object.assign(form, {
    Id: row.Id, Mode: row.Mode, Remark: row.Remark || '',
    Host: host || '127.0.0.1', Port: Number(port) || 8443,
    ConnectAddr: row.ConnectAddr || '', WsConnectAddr: row.WsConnectAddr || '',
    DNSDomain: row.DNSDomain || '', DisconnectTimeout: row.DisconnectTimeout ?? 60,
    PingInterval: row.PingInterval ?? 1, Vkey: row.Vkey || '', EncryptSalt: row.EncryptSalt || '',
  })
  open.value = true
}

async function submit() {
  // 走 antd 表单校验：必填项的提示文案由 zh_CN locale 提供（「请输入Vkey」）
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  if (!form.Vkey) { message.warning('Vkey 必填'); return }
  const body: any = {
    Mode: form.Mode, Vkey: form.Vkey, EncryptSalt: form.EncryptSalt, Remark: form.Remark,
    Host: form.Host, Port: form.Port, ConnectAddr: form.ConnectAddr,
    WsConnectAddr: form.WsConnectAddr, DNSDomain: form.DNSDomain,
    DisconnectTimeout: form.DisconnectTimeout, PingInterval: form.PingInterval,
  }
  if (editing.value) {
    await post('/listener/edit', { Id: editing.value, ...body })
    message.success('修改成功')
  } else {
    await post('/listener/add', body)
    message.success('新增成功')
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
    title: '是否确认删除监听？',
    onOk: async () => { await post('/listener/del', { id: row.Id }); message.success('已删除'); load() },
  })
}

function removeBatch() {
  if (!checkedKeys.value.length) { message.warning('先勾选监听器'); return }
  Modal.confirm({
    title: `删除选中的 ${checkedKeys.value.length} 个监听器？`,
    onOk: async () => {
      await post('/listener/dellist', { id: checkedKeys.value })
      message.success('已删除'); checkedKeys.value = []; load()
    },
  })
}
</script>

<template>
  <a-card :bordered="false">
    <div class="toolbar">
      <a-button type="primary" @click="create">新增监听</a-button>
      <a-button @click="load">刷新</a-button>
      <a-button danger :disabled="!checkedKeys.length" @click="removeBatch">删除</a-button>
      <span class="hint">共 {{ total }} 个</span>
    </div>

    <a-table
      :columns="columns" :data-source="rows" :loading="loading" row-key="Id" size="small" :pagination="false"
      :scroll="{ x: 1200 }"
      :row-selection="{ selectedRowKeys: checkedKeys, onChange: (keys: number[]) => (checkedKeys = keys) }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'Status'">
          <a-tag :color="record.Status ? 'green' : 'default'">{{ record.Status ? '运行' : '停止' }}</a-tag>
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a @click="toggle(record)">{{ record.Status ? '停用' : '启用' }}</a>
            <a @click="edit(record)">编辑</a>
            <a class="danger" @click="remove(record)">删除</a>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="open" :title="editing ? '编辑监听' : '新增监听'" :width="560" @ok="submit">
      <a-form ref="formRef" :model="form" :label-col="{ style: 'width: 130px' }" :wrapper-col="{ span: 15 }">
        <a-form-item label="模式">
          <a-select v-model:value="form.Mode">
            <a-select-option v-for="m in modes" :key="m" :value="m">{{ m }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="备注"><a-input v-model:value="form.Remark" /></a-form-item>
        <a-form-item label="监听地址" name="Host" :rules="[{ required: true, message: '请输入监听地址' }]">
          <a-input v-model:value="form.Host" placeholder="监听 IP，如 0.0.0.0" />
        </a-form-item>
        <a-form-item label="监听端口">
          <a-input-number v-model:value="form.Port" :min="1" :max="65535" style="width: 100%" />
        </a-form-item>
        <a-form-item label="外网连接地址">
          <a-input v-model:value="form.ConnectAddr" placeholder="客户端回连地址（可空，默认取监听地址）" />
        </a-form-item>
        <a-form-item label="ws连接地址">
          <a-input v-model:value="form.WsConnectAddr" placeholder="ws 模式专用（可空）" />
        </a-form-item>
        <a-form-item label="NS域名">
          <a-input v-model:value="form.DNSDomain" placeholder="dns 模式专用（可空）" />
        </a-form-item>
        <a-form-item label="超时心跳次数">
          <a-input-number v-model:value="form.DisconnectTimeout" :min="1" style="width: 100%" />
        </a-form-item>
        <a-form-item label="心跳间隔">
          <a-input-number v-model:value="form.PingInterval" :min="1" style="width: 100%" />
        </a-form-item>
        <!-- 4.9.3 的 helpMessage 是「通信凭证，通信的校验密码」；这里放进 placeholder，
             否则 help 会占掉 explain 区域，必填校验文案显示不出来 -->
        <a-form-item label="Vkey" name="Vkey" :rules="[{ required: true, message: '请输入Vkey' }]">
          <a-input-password v-model:value="form.Vkey" placeholder="通信凭证，通信的校验密码" />
        </a-form-item>
        <a-form-item label="流量加密盐">
          <a-input-password v-model:value="form.EncryptSalt" />
        </a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.hint { color: rgba(0, 0, 0, 0.45); }
.danger { color: #ff4d4f; }
</style>
