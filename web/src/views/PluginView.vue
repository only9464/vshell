<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { runnerApi } from '../api'

const route = useRoute()
const id = Number(route.params.id)
const plugins = ref<{ id: number; name: string }[]>([])
const output = ref('')
const procArg = ref('')
const filter = ref('')
const filtered = computed(() => {
  const q = filter.value.trim().toLowerCase()
  return q ? plugins.value.filter((p) => p.name.toLowerCase().includes(q)) : plugins.value
})
const loading = ref(false)

async function load() {
  plugins.value = (await runnerApi.list()) || []
}
onMounted(load)

async function copyOut() {
  if (!output.value) return ElMessage.warning('没有可复制的内容')
  try {
    await navigator.clipboard.writeText(output.value)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择文本')
  }
}

async function run(name: string) {
  loading.value = true
  output.value = ''
  try {
    const res = await runnerApi.run(id, name, procArg.value)
    output.value = (res as any)?.output ?? JSON.stringify(res, null, 2)
  } catch (e: any) {
    output.value = e?.message || String(e)
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="page">
    <div class="toolbar">
      <span class="mono">客户端 #{{ id }}</span>
      <el-input v-model="procArg" placeholder="插件参数（可选）" style="width: 260px" />
      <el-input v-model="filter" placeholder="筛选插件名" style="width: 200px" clearable />
      <el-button @click="load">刷新列表</el-button>
      <span class="mono">共 {{ plugins.length }} 个</span>
    </div>
    <el-row :gutter="12">
      <el-col :span="8">
        <el-card>
          <template #header>服务器插件（plugins/）</template>
          <el-table :data="filtered" size="small" v-loading="loading">
            <el-table-column prop="name" label="名称" />
            <template #empty>
              <el-empty :description="loading ? '加载中…' : 'plugins/ 目录下没有插件'" />
            </template>
            <el-table-column width="80">
              <template #default="{ row }">
                <el-button link type="primary" @click="run(row.name)">执行</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-col>
      <el-col :span="16">
        <el-card>
          <template #header>
            输出
            <el-button link type="primary" style="float: right" @click="output = ''">清空</el-button>
            <el-button link type="primary" style="float: right; margin-right: 8px" @click="copyOut">复制</el-button>
          </template>
          <pre class="out">{{ output || '（无输出）' }}</pre>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.out { min-height: 240px; margin: 0; white-space: pre-wrap; font-family: ui-monospace, monospace; }
</style>
