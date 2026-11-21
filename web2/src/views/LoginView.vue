<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const form = reactive({ username: 'admin', password: '' })

async function submit() {
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    router.replace(String(route.query.redirect || '/dashboard/index'))
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-page flex-center">
    <a-card class="login-card" :bordered="false">
      <div class="title">管理平台</div>
      <a-form :model="form" layout="vertical" @finish="submit">
        <a-form-item label="用户名" name="username">
          <a-input v-model:value="form.username" size="large" placeholder="用户名" />
        </a-form-item>
        <a-form-item label="密码" name="password">
          <a-input-password v-model:value="form.password" size="large" placeholder="密码" @press-enter="submit" />
        </a-form-item>
        <a-button type="primary" size="large" block :loading="loading" @click="submit">登录</a-button>
      </a-form>
    </a-card>
  </div>
</template>

<style scoped>
.login-page { height: 100%; background: #f0f2f5; }
.login-card { width: 360px; box-shadow: 0 6px 24px rgba(0, 0, 0, 0.08); }
.title { font-size: 20px; font-weight: 600; text-align: center; margin-bottom: 20px; }
</style>
