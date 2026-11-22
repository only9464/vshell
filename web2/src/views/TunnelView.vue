<script setup lang="ts">
// 隧道：列表 + 新建/编辑 + 启停/删除
import { onMounted, reactive, ref } from 'vue'
import { message, Modal } from 'ant-design-vue'
import { post } from '../api/http'

const rows = ref<any[]>([])
const loading = ref(false)
const open = ref(false)
const editing = ref<number | null>(null)
const clients = ref<any[]>([])
const form = reactive({
  clientId: 0, Port: 28001, Mode: 'tcp', Target: '127.0.0.1:22', Remark: '',
  Username: '', Password: '',
})

// 4.9.3 的四种隧道协议；后端目前只实现 TCP 代理，其余置灰并给出说明
// 4.9.3 的顺序与文案：Socks5隧道 / HTTP隧道 / TCP代理 / UDP代理
// 后端目前只实现 TCP 代理，其余置灰标注「未实现」
const protocols = [
  { label: 'Socks5隧道', value: 'socks5', disabled: true },
  { label: 'HTTP隧道', value: 'http', disabled: true },
  { label: 'TCP代理', value: 'tcp', disabled: false },
  { label: 'UDP代理', value: 'udp', disabled: true },
]

// 列定义取自 4.9.3 bundle（vCTMULPY8.js）
const columns = [
  { title: 'ID', dataIndex: 'Id', width: 70 },
  { title: '客户端 ID', dataIndex: 'client_id', width: 100 },
  { title: '备注', dataIndex: 'Remark', width: 120 },
  { title: '模式', dataIndex: 'Mode', width: 110 },
  { title: '端口', dataIndex: 'Port', width: 90 },
  { title: '目标 (IP:端口)', dataIndex: 'Target', width: 190 },
  { title: '用户名', dataIndex: 'username', width: 100 },
  { title: '密码', dataIndex: 'password', width: 100 },
  { title: '隧道状态', dataIndex: 'RunStatus', width: 100 },
  { title: '客户端状态', dataIndex: 'clientOnline', width: 110 },
  { title: '操作', key: 'action', width: 180, fixed: 'right' },
]

async function load() {
  loading.value = true
  try {
    const [t, c]: any[] = await Promise.all([
      post('/tunnel/list', { page: 1, pageSize: 100 }),
      post('/client/list', { page: 1, pageSize: 100 }),
    ])
    rows.value = t?.result?.items || []
    clients.value = c?.result?.items || []
  } finally { loading.value = false }
}
onMounted(load)

function create() {
  editing.value = null
  Object.assign(form, { clientId: clients.value[0]?.Id || 0, Port: 28001, Mode: 'tcp', Target: '127.0.0.1:22', Remark: '' })
  open.value = true
}

function edit(row: any) {
  editing.value = row.Id
  Object.assign(form, { clientId: row.client_id, Port: row.Port, Mode: row.Mode, Target: row.Target, Remark: row.Remark || '' })
  open.value = true
}

async function submit() {
  if (editing.value) await post('/tunnel/edit', { Id: editing.value, ...form })
  else await post('/tunnel/add', { ...form })
  message.success('已保存'); open.value = false; load()
}

async function toggle(row: any) {
  await post(row.RunStatus ? '/tunnel/stop' : '/tunnel/start', { id: row.Id })
  load()
}

function remove(row: any) {
  Modal.confirm({
    title: '是否确认删除隧道代理？',
    onOk: async () => { await post('/tunnel/del', { id: row.Id }); message.success('已删除'); load() },
  })
}
</script>

<template>
  <a-card :bordered="false">
    <div class="toolbar">
      <a-button type="primary" @click="create">新增隧道代理</a-button>
      <a-button @click="load">刷新</a-button>
    </div>
    <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="Id" size="small" :pagination="false" :scroll="{ x: 1100 }">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'RunStatus'">
          <a-tag :color="record.RunStatus ? 'green' : 'default'">{{ record.RunStatus ? '运行' : '停止' }}</a-tag>
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a @click="toggle(record)">{{ record.RunStatus ? '停用' : '启用' }}</a>
            <a @click="edit(record)">编辑</a>
            <a class="danger" @click="remove(record)">删除</a>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal v-model:open="open" :title="editing ? '编辑隧道代理' : '新增隧道代理'" :width="520" @ok="submit">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="客户端">
          <a-select v-model:value="form.clientId">
            <a-select-option v-for="c in clients" :key="c.Id" :value="c.Id">
              #{{ c.Id }} {{ c.HostName || c.LocalIP }}
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="服务器端口"><a-input-number v-model:value="form.Port" :min="1" :max="65535" style="width:100%" /></a-form-item>
        <a-form-item label="模式">
          <a-radio-group v-model:value="form.Mode">
            <a-radio-button v-for="p in protocols" :key="p.value" :value="p.value" :disabled="p.disabled">
              {{ p.label }}{{ p.disabled ? '（未实现）' : '' }}
            </a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item v-if="form.Mode !== 'socks5' && form.Mode !== 'http'" label="目标(IP:端口)" :help="'格式：10.1.1.99:80'">
          <a-input v-model:value="form.Target" placeholder="10.1.1.99:80" />
        </a-form-item>
        <a-form-item v-if="form.Mode === 'socks5' || form.Mode === 'http'" label="用户名">
          <a-input v-model:value="form.Username" />
        </a-form-item>
        <a-form-item v-if="form.Mode === 'socks5' || form.Mode === 'http'" label="密码">
          <a-input-password v-model:value="form.Password" />
        </a-form-item>
        <a-form-item label="备注"><a-input v-model:value="form.Remark" /></a-form-item>
      </a-form>
    </a-modal>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; margin-bottom: 12px; }
.danger { color: #ff4d4f; }
</style>
