<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { get, post } from '../api/http'

const loading = ref(false)
const form = reactive({ wx_key: '', dingding_access_token: '', dingding_key_word: '' })

async function load() {
  loading.value = true
  try {
    const res: any = await get('/setting/get')
    Object.assign(form, res?.result || {})
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function save() {
  await post('/setting/edit', { ...form })
  message.success('已保存')
  load()
}
</script>

<template>
  <a-card :bordered="false" title="设置" :loading="loading">
    <a-form :label-col="{ span: 5 }" :wrapper-col="{ span: 12 }">
      <a-form-item label="企业微信 key"><a-input v-model:value="form.wx_key" placeholder="留空表示不启用" /></a-form-item>
      <a-form-item label="钉钉 access_token"><a-input v-model:value="form.dingding_access_token" /></a-form-item>
      <a-form-item label="钉钉关键字"><a-input v-model:value="form.dingding_key_word" /></a-form-item>
      <a-form-item :wrapper-col="{ offset: 5 }">
        <a-button type="primary" @click="save">保存</a-button>
        <span class="hint">通知功能需要第三方凭证，未配置时不影响其它功能</span>
      </a-form-item>
    </a-form>
  </a-card>
</template>

<style scoped>
.hint { margin-left: 12px; color: rgba(0, 0, 0, 0.45); font-size: 12px; }
</style>
