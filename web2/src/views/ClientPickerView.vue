<script setup lang="ts">
// 4.9.3 的 /terminal/index 与 /filemanager/index：先选客户端，再进具体页面。
// 这里做成一张客户端表格（列与客户端列表一致的精简版），点「进入」跳对应路由。
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { post } from '../api/http'

const props = defineProps<{ kind: 'terminal' | 'filemanager' }>()
const router = useRouter()
const rows = ref<any[]>([])
const loading = ref(false)

const title = computed(() => (props.kind === 'terminal' ? '命令控制' : '文件管理'))
const target = computed(() => (props.kind === 'terminal' ? '/terminal' : '/filemanager'))

const columns = [
  { title: 'ID', dataIndex: 'Id', width: 70 },
  { title: '备注', dataIndex: 'Remark', width: 140 },
  { title: '连接方式', dataIndex: 'Tp', width: 100 },
  { title: '外网IP', dataIndex: 'Addr', width: 170 },
  { title: '主机名', dataIndex: 'HostName', width: 160 },
  { title: '系统类型', dataIndex: 'OsName', width: 140 },
  { title: '状态', dataIndex: 'IsConnect', width: 90 },
  { title: '操作', key: 'action', width: 120 },
]

async function load() {
  loading.value = true
  try {
    const res: any = await post('/client/list', { page: 1, pageSize: 200 })
    rows.value = res?.result?.items || []
  } finally {
    loading.value = false
  }
}
onMounted(load)

function enter(row: any) {
  router.push(`${target.value}/${row.Id}`)
}
</script>

<template>
  <a-card :bordered="false" :title="title">
    <div class="toolbar">
      <span class="hint">选择一个客户端进入{{ title }}（4.9.3 的 {{ target }}/index 行为）</span>
      <a-button style="margin-left: auto" @click="load">刷新</a-button>
    </div>
    <a-table :columns="columns" :data-source="rows" :loading="loading" row-key="Id" size="small" :pagination="false">
      <template #bodyCell="{ column, record }">
        <template v-if="column.dataIndex === 'IsConnect'">
          <a-tag :color="record.IsConnect ? 'green' : 'default'">{{ record.IsConnect ? '在线' : '离线' }}</a-tag>
        </template>
        <template v-else-if="column.key === 'action'">
          <a @click="enter(record)">进入</a>
        </template>
      </template>
    </a-table>
  </a-card>
</template>

<style scoped>
.toolbar { display: flex; align-items: center; margin-bottom: 12px; }
.hint { color: rgba(0, 0, 0, 0.45); font-size: 13px; }
</style>
