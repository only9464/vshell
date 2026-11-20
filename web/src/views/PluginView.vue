<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'
import { runnerApi } from '../api'

const route = useRoute()
const id = Number(route.params.id)
const plugins = ref<{ id: number; name: string }[]>([])
const output = ref('')
const procArg = ref('')
const loading = ref(false)

async function load() {
  plugins.value = (await runnerApi.list()) || []
}
onMounted(load)

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
      <el-button @click="load">刷新列表</el-button>
    </div>
    <el-row :gutter="12">
      <el-col :span="8">
        <el-card>
          <template #header>服务器插件（plugins/）</template>
          <el-table :data="plugins" size="small" v-loading="loading">
            <el-table-column prop="name" label="名称" />
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
          <template #header>输出</template>
          <pre class="out">{{ output || '（无输出）' }}</pre>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<style scoped>
.out { min-height: 240px; margin: 0; white-space: pre-wrap; font-family: ui-monospace, monospace; }
</style>
