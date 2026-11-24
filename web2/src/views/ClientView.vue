<script setup lang="ts">
// 客户端管理：对齐 4.9.3 的表格列与操作（Ant Design Vue 实现）。
// 列：ID / 状态 / 主机名 / 用户 / 系统 / 通道 / 外网地址 / 内网 IP / 进程 / 备注 / 操作
import { h, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { downloadURL, post } from '../api/http'

const router = useRouter()
const loading = ref(false)
const rows = ref<any[]>([])
const total = ref(0)
const online = ref(0)
const keyword = ref('')
// 4.9.3 的搜索表单（vBWgKrY3R.js / searchFormSchema）：关键词 + 状态（在线/离线）
const status = ref<string | undefined>(undefined)
// 搜索表单默认收起、批量选择默认关闭（4.9.3 的 useSearchForm:false / rowSelection:false）
const showSearch = ref(false)
const batchSelect = ref(false)
const forwardOpen = ref(false)
const forward = reactive({ ebpf: false, tp: 'tcp', ip: '', port: 4444, ws: '', vkey: '', salt: '', proxy: '' })

function toggleBatchSelect() {
  batchSelect.value = !batchSelect.value
  if (!batchSelect.value) checkedKeys.value = []
}

function resetSearch() {
  keyword.value = ''
  status.value = undefined
  load()
}
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

// 行内下拉与右键菜单共用同一套动作
function ctxActionFor(row: any, key: string) {
  ctxRow.value = row
  ctxAction(key)
}

function ctxAction(key: string) {
  ctxOpen.value = false
  const row = ctxRow.value
  if (!row) return
  if (key === 'manage') {
    // 4.9.3：跳 /clientManager/:id（主机管理，即文件 + 终端入口）
    router.push(`/clientManager/files/${row.Id}`)
  } else if (key === 'tunnel') {
    // 4.9.3：跳 /tunnel/:id
    router.push(`/tunnel/${row.Id}`)
  } else if (key === 'delfile') {
    Modal.confirm({
      title: '温馨提示',
      content: `是否确认删除 ID:${row.Id} 客户端进程文件`,
      onOk: async () => {
        try { await post('/client/delfile', { id: row.Id }); message.success('删除进程文件成功') }
        catch { message.error('删除进程文件失败') }
      },
    })
  } else if (key === 'delhost') {
    Modal.confirm({
      title: '温馨提示',
      content: `是否确认下线 ID:${row.Id} 客户端`,
      onOk: async () => {
        try { await post('/client/delprocess', { id: row.Id }); message.success('下线成功') }
        catch { message.error('下线失败') }
        load()
      },
    })
  }
}
const page = ref(1)
const pageSize = ref(20)

// 列定义与顺序完全取自 4.9.3 的 bundle（viLthP3eG.js）
const columns = [
  { title: 'ID', dataIndex: 'Id', width: 56, sorter: (a: any, b: any) => Number(a.Id || 0) - Number(b.Id || 0) },
  { title: '备注', dataIndex: 'Remark', width: 90, sorter: (a: any, b: any) => String(a.Remark || '').localeCompare(String(b.Remark || '')) },
  { title: '连接方式', dataIndex: 'Tp', width: 76 },
  { title: '外网IP', dataIndex: 'Addr', width: 130, sorter: (a: any, b: any) => String(a.Addr || '').localeCompare(String(b.Addr || '')) },
  { title: '归属地', dataIndex: 'Location', width: 76, sorter: (a: any, b: any) => String(a.Location || '').localeCompare(String(b.Location || '')) },
  { title: '内网IP', dataIndex: 'LocalIP', width: 108, sorter: (a: any, b: any) => String(a.LocalIP || '').localeCompare(String(b.LocalIP || '')) },
  { title: '用户', dataIndex: 'UserName', width: 64 },
  { title: '主机名', dataIndex: 'HostName', width: 92, sorter: (a: any, b: any) => String(a.HostName || '').localeCompare(String(b.HostName || '')) },
  { title: '系统类型', dataIndex: 'OsName', width: 96, sorter: (a: any, b: any) => String(a.OsName || '').localeCompare(String(b.OsName || '')) },
  { title: '进程名', dataIndex: 'ProcessName', width: 88 },
  { title: '状态', dataIndex: 'IsConnect', width: 56 },
  { title: '心跳', dataIndex: 'PingCheckTime', width: 64 },
  { title: '操作', key: 'action', width: 170, fixed: 'right' },
]

// 行内编辑备注（4.9.3 的 Remark 列 edit:!0 → /client/editremark）
function inlineEditRemark(row: any) {
  let v = row.Remark || ''
  Modal.confirm({
    title: `编辑 ID:${row.Id} 的备注`,
    content: () => h('input', {
      class: 'ant-input', value: v, style: 'width:100%',
      onInput: (e: Event) => { v = (e.target as HTMLInputElement).value },
    }),
    onOk: async () => {
      await post('/client/editremark', { id: row.Id, remark: v })
      message.success('修改成功')
      load()
    },
  })
}

// 正向连接（4.9.3 的 AddClientDrawer）：按 ebpf/连接方式/地址 生成倾听型载荷
function submitForward() {
  if (!forward.vkey) { message.warning('请输入 Vkey'); return }
  const host = forward.tp === 'ws' ? forward.ws : forward.ip
  if (!host) { message.warning(forward.tp === 'ws' ? '请输入 WS 地址' : '请输入 IP 或域名'); return }
  const arch = forward.ebpf ? 'linux_amd64' : 'windows_amd64'
  const url = downloadURL('/download/listen', {
    arch, tp: forward.tp === 'ws' ? 'ws' : forward.tp,
    host: forward.ip || '0.0.0.0', port: forward.port,
    vkey: forward.vkey, salt: forward.salt, upx: 'false',
    listen: 'true', ebpf: String(forward.ebpf),
  })
  location.href = url
  forwardOpen.value = false
}

async function load() {
  loading.value = true
  try {
    // 注意：4.9.3 的业务接口几乎都是 POST（只有少数 GET），这里统一用 post。
    const body: any = { page: page.value, pageSize: pageSize.value, search: keyword.value }
    // 状态：1=在线 2=离线（与 4.9.3 的选项值一致）
    if (status.value) body.status = Number(status.value)
    const res: any = await post('/client/list', body)
    rows.value = res?.result?.items || []
    total.value = res?.result?.total || 0
    online.value = res?.result?.clientOnlineCount || 0
  } finally {
    loading.value = false
  }
}
onMounted(load)

// 心跳：后端给的是"最后一次心跳的时间戳"（Unix 秒）。4.9.3 的规则是
// 「<60 绿色，否则红色」——这里换算成"距今多少秒"再套同一条规则。
function hbAge(v: any): number {
  const n = Number(v)
  if (!n) return -1
  const secs = n > 1e12 ? Math.floor(n / 1000) : n
  const now = Math.floor(Date.now() / 1000)
  return Math.max(0, now - secs)
}

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
    title: '温馨提示',
    content: '是否确认删除已选中的客户端，包括文件和进程',
    onOk: async () => {
      try {
        await post('/client/dellist', { id: checkedKeys.value })
        message.success('批量删除成功')
      } catch { message.error('批量删除失败') }
      checkedKeys.value = []
      batchSelect.value = false
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
    <!-- 工具栏：搜索 / 批量选择 / 批量删除 / 正向连接（文案、图标、出现条件照 4.9.3
         的 vBWgKrY3R.js：only 选中时出现"批量删除"，"正向连接"在最右） -->
    <div class="toolbar">
      <a-button type="primary" @click="showSearch = !showSearch">
        <span class="act i-ant-design:search-outlined" />搜索
      </a-button>
      <a-button @click="toggleBatchSelect">
        <span class="act i-ant-design:select-outlined" />批量选择
      </a-button>
      <a-button v-if="checkedKeys.length" danger @click="removeBatch">
        <span class="act i-ant-design:delete-outlined" />批量删除
      </a-button>
      <a-button type="primary" class="ml-auto" @click="forwardOpen = true">
        <span class="act i-ant-design:plus-outlined" />正向连接
      </a-button>
      <span class="mono hint">共 {{ total }} 台，在线 {{ online }}</span>
    </div>

    <div v-if="showSearch" class="searchbar">
      <span class="lbl">关键词</span>
      <a-input v-model:value="keyword" placeholder="主机/用户/备注" style="width: 200px"
        allow-clear @press-enter="load" />
      <span class="lbl">状态</span>
      <a-select v-model:value="status" allow-clear placeholder="全部" style="width: 110px" @change="load">
        <a-select-option value="1">在线</a-select-option>
        <a-select-option value="2">离线</a-select-option>
      </a-select>
      <a-button type="primary" @click="load">查询</a-button>
      <a-button @click="resetSearch">重置</a-button>
    </div>

    <a-table
      :columns="columns" :data-source="rows" :loading="loading" row-key="Id" size="small"
      :scroll="{ x: 1400 }"
      :row-selection="batchSelect ? { selectedRowKeys: checkedKeys, onChange: (keys: number[]) => (checkedKeys = keys) } : undefined"
      :custom-row="(record: any) => ({ onContextmenu: (e: MouseEvent) => openCtx(e, record) })"
      :pagination="{ current: page, pageSize, total, showSizeChanger: true, showTotal: (t: number) => `共 ${t} 台` }"
      @change="(p: any) => { page = p.current; pageSize = p.pageSize; load() }"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'OsName'">
          <span v-if="String(record.OsName || '').startsWith('windows')">
            <span class="os-ico i-devicon:windows8" />{{ String(record.OsName).replace('windows_', ' ') }}
          </span>
          <span v-else-if="String(record.OsName || '').startsWith('linux')">
            <span class="os-ico i-devicon:linux" />{{ String(record.OsName).replace('linux_', ' ') }}
          </span>
          <span v-else-if="String(record.OsName || '').startsWith('darwin')">
            <span class="os-ico i-wpf:macos" />{{ String(record.OsName).replace('darwin_', ' ') }}
          </span>
          <span v-else>{{ record.OsName || '-' }}</span>
        </template>
        <template v-else-if="column.dataIndex === 'PingCheckTime'">
          <a-tag v-if="hbAge(record.PingCheckTime) >= 0"
            :color="hbAge(record.PingCheckTime) < 60 ? 'green' : 'red'">
            {{ hbAge(record.PingCheckTime) }}s
          </a-tag>
          <span v-else>-</span>
        </template>
        <template v-else-if="column.dataIndex === 'Remark'">
          <a-tooltip title="点击编辑备注">
            <span class="editable" @click="inlineEditRemark(record)">{{ record.Remark || '—' }}</span>
          </a-tooltip>
        </template>
        <template v-else-if="column.dataIndex === 'IsConnect'">
          <a-tag :color="record.IsConnect ? 'green' : 'default'">{{ record.IsConnect ? '在线' : '离线' }}</a-tag>
        </template>
        <template v-else-if="column.dataIndex === 'HostName'">
          <a @click="open(record)">{{ record.HostName || '(未知)' }}</a>
        </template>
        <template v-else-if="column.key === 'action'">
          <!-- 4.9.3：行内只放两个（主机管理 / 隧道代理，仅在线），其余进下拉 -->
          <a-space size="small">
            <template v-if="record.IsConnect">
              <a-tooltip title="主机管理"><a @click="router.push(`/clientManager/files/${record.Id}`)">
                <span class="act i-grommet-icons:drag" /></a></a-tooltip>
              <a-tooltip title="隧道代理"><a @click="router.push(`/tunnel/${record.Id}`)">
                <span class="act i-fa6-solid:globe" /></a></a-tooltip>
            </template>
            <a-dropdown :trigger="['click']">
              <a><span class="act i-ant-design:ellipsis-vertical" /></a>
              <template #overlay>
                <a-menu @click="(e: any) => ctxActionFor(record, String(e.key))">
                  <a-menu-item v-if="record.IsConnect" key="delfile">
                    <span class="act i-grommet-icons:trash" />删除文件
                  </a-menu-item>
                  <a-menu-item key="delhost">
                    <span class="act i-grommet-icons:close" />删除主机
                  </a-menu-item>
                </a-menu>
              </template>
            </a-dropdown>
          </a-space>
        </template>
      </template>
    </a-table>

    <!-- 正向连接（4.9.3 的 AddClientDrawer 字段：ebpf / 连接方式 / IP 或域名 / 端口 /
         WS 地址 / Vkey / 流量加密盐 / 使用代理） -->
    <a-modal v-model:open="forwardOpen" title="正向连接" :width="520" @ok="submitForward">
      <a-form :label-col="{ style: 'width: 110px' }" :wrapper-col="{ span: 14 }">
        <a-form-item label="ebpf客户端">
          <a-radio-group v-model:value="forward.ebpf">
            <a-radio-button :value="true">是</a-radio-button>
            <a-radio-button :value="false">否</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item label="连接方式">
          <a-radio-group v-model:value="forward.tp">
            <a-radio-button value="tcp">tcp</a-radio-button>
            <a-radio-button value="kcp">kcp</a-radio-button>
            <a-radio-button value="ws">ws</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item v-if="forward.tp !== 'ws'" label="IP或域名">
          <a-input v-model:value="forward.ip" placeholder="监听地址，如 0.0.0.0" />
        </a-form-item>
        <a-form-item v-if="forward.tp !== 'ws'" label="端口">
          <a-input-number v-model:value="forward.port" :min="1" :max="65535" style="width: 100%" />
        </a-form-item>
        <a-form-item v-if="forward.tp === 'ws'" label="WS地址">
          <a-input v-model:value="forward.ws" placeholder="格式：ws://192.168.1.1:8080/ws，支持 wss" />
        </a-form-item>
        <a-form-item label="Vkey"><a-input v-model:value="forward.vkey" placeholder="通信凭证，通信的校验密码" /></a-form-item>
        <a-form-item label="流量加密盐"><a-input v-model:value="forward.salt" /></a-form-item>
        <a-form-item label="使用代理"><a-input v-model:value="forward.proxy" placeholder="不使请留空" /></a-form-item>
      </a-form>
    </a-modal>

    <!-- 右键菜单：文案 / 图标 / 顺序 / 确认框文本全部照 4.9.3 的
         vBWgKrY3R.js（主机管理 / 隧道 / 删除文件 / 删除主机，前三项仅在线显示） -->
    <a-dropdown :open="ctxOpen" :trigger="[]" @openChange="(v: boolean) => (ctxOpen = v)">
      <span :style="{ position: 'fixed', left: ctxPos.x + 'px', top: ctxPos.y + 'px', width: 1, height: 1 }" />
      <template #overlay>
        <a-menu @click="(e: any) => ctxAction(String(e.key))">
          <a-menu-item v-if="ctxRow && ctxRow.IsConnect" key="manage">
            <span class="act i-grommet-icons:drag" />主机管理
          </a-menu-item>
          <a-menu-item v-if="ctxRow && ctxRow.IsConnect" key="tunnel">
            <span class="act i-fa6-solid:globe" />隧道代理
          </a-menu-item>
          <a-menu-item v-if="ctxRow && ctxRow.IsConnect" key="delfile">
            <span class="act i-grommet-icons:trash" />删除文件
          </a-menu-item>
          <a-menu-item key="delhost">
            <span class="act i-grommet-icons:close" />删除主机
          </a-menu-item>
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
.searchbar { display: flex; align-items: center; gap: 8px; margin: 0 0 12px; }
.ml-auto { margin-left: auto; }
.lbl { color: rgba(0, 0, 0, 0.65); font-size: 13px; }
.danger { color: #ff4d4f; }
.os-ico { width: 16px; height: 16px; display: inline-block; vertical-align: -3px; margin-right: 4px; }
.editable { cursor: text; border-bottom: 1px dashed rgba(0,0,0,.25); }
.act { width: 14px; height: 14px; display: inline-block; vertical-align: -2px; margin-right: 3px; }
</style>
