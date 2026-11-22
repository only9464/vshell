<script setup lang="ts">
// 客户端管理：对齐 4.9.3 的表格列与操作（Ant Design Vue 实现）。
// 列：ID / 状态 / 主机名 / 用户 / 系统 / 通道 / 外网地址 / 内网 IP / 进程 / 备注 / 操作
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { post } from '../api/http'

const router = useRouter()
const loading = ref(false)
const rows = ref<any[]>([])
const total = ref(0)
const online = ref(0)
const keyword = ref('')
const selected = ref<any>(null)
const drawer = ref(false)
const checkedKeys = ref<number[]>([])
// 右键菜单（4.9.3 的列表交互）
const ctxOpen = ref(false)
const ctxRow = ref<any>(null)
const ctxPos = ref({ x: 0, y: 0 })

function openCtx(e: MouseEvent, row: any) {
  e.preventDefault()
  ctxRow.value = row
  ctxPos.value = { x: e.clientX, y: e.clientY }
  ctxOpen.value = true
}

function ctxAction(key: string) {
  ctxOpen.value = false
  const row = ctxRow.value
  if (!row) return
  if (key === 'terminal') router.push(`/clientManager/terminal/${row.Id}`)
  else if (key === 'files') router.push(`/clientManager/files/${row.Id}`)
  else if (key === 'detail') open(row)
  else if (key === 'remark') editRemark(row)
  else if (key === 'delete') remove(row)
}
const page = ref(1)
const pageSize = ref(20)

const columns = [
  // 列名取自 4.9.3 locale：客户端 ID / 客户端状态 / 主机名 / 用户 / 系统类型 /
  // 外网IP / 内网IP / 归属地 / 进程名 / 备注 / 操作
  { title: '客户端 ID', dataIndex: 'Id', width: 100 },
  { title: '客户端状态', dataIndex: 'IsConnect', width: 110 },
  { title: '主机名', dataIndex: 'HostName', width: 140 },
  { title: '用户', dataIndex: 'UserName', width: 110 },
  { title: '系统类型', dataIndex: 'OsName', width: 110 },
  { title: '架构', dataIndex: 'Arch', width: 90 },
  { title: '连接方式', dataIndex: 'Tp', width: 100 },
  { title: '外网IP', dataIndex: 'Addr', width: 150 },
  { title: '内网IP', dataIndex: 'LocalIP', width: 130 },
  { title: '归属地', dataIndex: 'Location', width: 110 },
  { title: '进程名', dataIndex: 'ProcessName', width: 120 },
  { title: '备注', dataIndex: 'Remark' },
  { title: '操作', key: 'action', width: 260, fixed: 'right' },
]

async function load() {
  loading.value = true
  try {
    // 注意：4.9.3 的业务接口几乎都是 POST（只有少数 GET），这里统一用 post。
    const res: any = await post('/client/list', { page: page.value, pageSize: pageSize.value, search: keyword.value })
    rows.value = res?.result?.items || []
    total.value = res?.result?.total || 0
    online.value = res?.result?.clientOnlineCount || 0
  } finally {
    loading.value = false
  }
}
onMounted(load)

function open(row: any) { selected.value = row; drawer.value = true }

async function editRemark(row: any) {
  const value = window.prompt('备注', row.Remark || '')
  if (value === null) return
  await post('/client/editremark', { id: row.Id, remark: value })
  message.success('已保存')
  load()
}

// 批量删除（4.9.3 的列表多选 + 批量操作）
function removeBatch() {
  if (!checkedKeys.value.length) { message.warning('先勾选客户端'); return }
  Modal.confirm({
    title: `删除选中的 ${checkedKeys.value.length} 台客户端？`,
    onOk: async () => {
      await post('/client/dellist', { id: checkedKeys.value })
      message.success('已删除')
      checkedKeys.value = []
      load()
    },
  })
}

function remove(row: any) {
  Modal.confirm({
    title: `删除客户端 ${row.HostName || row.Id}？`,
    onOk: async () => {
      await post('/client/dellist', { id: [row.Id] })
      message.success('已删除')
      load()
    },
  })
}
</script>

<template>
  <a-card :bordered="false">
    <div class="toolbar">
      <a-input-search v-model:value="keyword" placeholder="搜索主机/用户/备注" style="width: 260px"
        allow-clear @search="load" @press-enter="load" />
      <a-button @click="load">刷新</a-button>
      <a-button danger :disabled="!checkedKeys.length" @click="removeBatch">删除</a-button>
      <span class="mono hint">共 {{ total }} 台，在线 {{ online }}</span>
    </div>

    <a-table
      :columns="columns" :data-source="rows" :loading="loading" row-key="Id" size="small"
      :scroll="{ x: 1400 }"
      :row-selection="{ selectedRowKeys: checkedKeys, onChange: (keys: number[]) => (checkedKeys = keys) }"
      :custom-row="(record: any) => ({ onContextmenu: (e: MouseEvent) => openCtx(e, record) })"
      :pagination="{ current: page, pageSize, total, showSizeChanger: true, showTotal: (t: number) => `共 ${t} 台` }"
      @change="(p: any) => { page = p.current; pageSize = p.pageSize; load() }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'IsConnect'">
          <a-tag :color="record.IsConnect ? 'green' : 'default'">{{ record.IsConnect ? '在线' : '离线' }}</a-tag>
        </template>
        <template v-else-if="column.dataIndex === 'HostName'">
          <a @click="open(record)">{{ record.HostName || '(未知)' }}</a>
        </template>
        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a-tooltip title="命令控制"><a @click="router.push(`/clientManager/terminal/${record.Id}`)">
              <span class="act i-material-icon-theme:console" />终端</a></a-tooltip>
            <a-tooltip title="文件管理"><a @click="router.push(`/clientManager/files/${record.Id}`)">
              <span class="act i-grommet-icons:folder" />文件</a></a-tooltip>
            <a-tooltip title="编辑备注"><a @click="editRemark(record)">
              <span class="act i-ant-design:edit-outlined" />编辑</a></a-tooltip>
            <a-tooltip title="删除主机"><a class="danger" @click="remove(record)">
              <span class="act i-grommet-icons:trash" />删除</a></a-tooltip>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-dropdown :open="ctxOpen" :trigger="[]" @openChange="(v: boolean) => (ctxOpen = v)">
      <span :style="{ position: 'fixed', left: ctxPos.x + 'px', top: ctxPos.y + 'px', width: 1, height: 1 }" />
      <template #overlay>
        <a-menu @click="(e: any) => ctxAction(String(e.key))">
          <a-menu-item key="terminal"><span class="act i-material-icon-theme:console" />命令控制</a-menu-item>
          <a-menu-item key="files"><span class="act i-grommet-icons:folder" />文件管理</a-menu-item>
          <a-menu-item key="detail"><span class="act i-ant-design:field-time" />查看详情</a-menu-item>
          <a-menu-item key="remark"><span class="act i-ant-design:edit-outlined" />编辑备注</a-menu-item>
          <a-menu-divider />
          <a-menu-item key="delete" danger><span class="act i-grommet-icons:trash" />删除主机</a-menu-item>
        </a-menu>
      </template>
    </a-dropdown>

    <a-drawer v-model:open="drawer" title="客户端详情" width="420">
      <a-descriptions v-if="selected" :column="1" bordered size="small">
        <a-descriptions-item label="ID">{{ selected.Id }}</a-descriptions-item>
        <a-descriptions-item label="Vkey">{{ selected.VerifyKey || '-' }}</a-descriptions-item>
        <a-descriptions-item label="通道">{{ selected.Tp }}</a-descriptions-item>
        <a-descriptions-item label="外网地址">{{ selected.Addr }}</a-descriptions-item>
        <a-descriptions-item label="内网 IP">{{ selected.LocalIP }}</a-descriptions-item>
        <a-descriptions-item label="主机名">{{ selected.HostName }}</a-descriptions-item>
        <a-descriptions-item label="用户">{{ selected.UserName }}</a-descriptions-item>
        <a-descriptions-item label="系统">{{ selected.OsName }}</a-descriptions-item>
        <a-descriptions-item label="架构">{{ selected.Arch }}</a-descriptions-item>
        <a-descriptions-item label="进程">{{ selected.ProcessName }}</a-descriptions-item>
        <a-descriptions-item label="归属地">{{ selected.Location || '-' }}</a-descriptions-item>
        <a-descriptions-item label="备注">{{ selected.Remark || '-' }}</a-descriptions-item>
        <a-descriptions-item label="在线">
          <a-tag :color="selected.IsConnect ? 'green' : 'default'">{{ selected.IsConnect ? '在线' : '离线' }}</a-tag>
        </a-descriptions-item>
      </a-descriptions>
      <template #extra>
        <a-space>
          <a-button size="small" @click="router.push(`/clientManager/terminal/${selected?.Id}`)">终端</a-button>
          <a-button size="small" @click="router.push(`/clientManager/files/${selected?.Id}`)">文件</a-button>
        </a-space>
      </template>
    </a-drawer>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; gap: 8px; align-items: center; margin-bottom: 12px; }
.hint { color: rgba(0, 0, 0, 0.45); }
.danger { color: #ff4d4f; }
.act { width: 14px; height: 14px; display: inline-block; vertical-align: -2px; margin-right: 3px; }
</style>
