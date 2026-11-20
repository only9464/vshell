<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { settingApi } from '../api'

const form = reactive({ wx_key: '', dingding_access_token: '', dingding_key_word: '' })
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const res = await settingApi.get()
    Object.assign(form, res || {})
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function save() {
  await settingApi.edit({ ...form })
  ElMessage.success('已保存到 conf/setting.conf')
}
</script>

<template>
  <div class="page" v-loading="loading">
    <el-card style="max-width: 620px">
      <template #header>通知设置</template>
      <el-form label-width="150px">
        <el-form-item label="企业微信 Key"><el-input v-model="form.wx_key" /></el-form-item>
        <el-form-item label="钉钉 AccessToken"><el-input v-model="form.dingding_access_token" /></el-form-item>
        <el-form-item label="钉钉关键词"><el-input v-model="form.dingding_key_word" /></el-form-item>
        <el-form-item>
          <el-button type="primary" @click="save">保存</el-button>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>
