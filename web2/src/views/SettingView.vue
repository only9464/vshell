<script setup lang="ts">
// 系统设置：字段与 4.9.3 的设置页完全一致（labelWidth=120，三项 Input，保存提示「设置成功」）
// 4.9.3 原文：钉钉 token / 钉钉 key_word / 微信 wx_key
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { get, post } from '../api/http'

const loading = ref(false)
const form = reactive({ dingding_access_token: '', dingding_key_word: '', wx_key: '' })

async function load() {
  loading.value = true
  try {
    const res: any = await get('/setting/get')
    const r = res?.result || {}
    form.dingding_access_token = r.dingding_access_token || ''
    form.dingding_key_word = r.dingding_key_word || ''
    form.wx_key = r.wx_key || ''
  } finally {
    loading.value = false
  }
}
onMounted(load)

async function save() {
  await post('/setting/edit', { ...form })
  message.success('设置成功')
  load()
}
</script>

<template>
  <a-card :bordered="false" title="系统设置" :loading="loading">
    <a-form :label-col="{ style: 'width: 120px' }" :wrapper-col="{ span: 12 }">
      <a-form-item label="钉钉 token">
        <a-input v-model:value="form.dingding_access_token" placeholder="不使请留空" />
      </a-form-item>
      <a-form-item label="钉钉 key_word">
        <a-input v-model:value="form.dingding_key_word" placeholder="不使请留空" />
      </a-form-item>
      <a-form-item label="微信 wx_key">
        <a-input v-model:value="form.wx_key" placeholder="不使请留空" />
      </a-form-item>
      <a-form-item :wrapper-col="{ offset: 0 }">
        <a-button type="primary" @click="save">保存</a-button>
        <span class="hint">通知功能需要第三方凭证，按需求暂不实现；此处仅保存配置</span>
      </a-form-item>
    </a-form>
  </a-card>
</template>

<style scoped>
.hint { margin-left: 12px; color: rgba(0, 0, 0, 0.45); font-size: 12px; }
</style>
