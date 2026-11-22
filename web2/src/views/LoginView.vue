<script setup lang="ts">
// 登录页对齐 4.9.3（vben-admin 5 风格）：
// 左侧品牌/渐变装饰栏 + 右侧表单卡片；占位文案取自 4.9.3 locale：
// 「请输入账号」「请输入密码」「记住我」「忘记密码?」。
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { UserOutlined, LockOutlined } from '@ant-design/icons-vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
const loading = ref(false)
const remember = ref(true)
const form = reactive({ username: 'admin', password: '' })

async function submit() {
  if (!form.username || !form.password) return
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    if (remember.value) localStorage.setItem('vshell.remember', '1')
    router.replace(String(route.query.redirect || '/dashboard/index'))
  } finally {
    loading.value = false
  }
}

const dark = ref(document.documentElement.getAttribute('data-theme') === 'dark')
function toggleTheme() {
  dark.value = !dark.value
  document.documentElement.setAttribute('data-theme', dark.value ? 'dark' : 'light')
  localStorage.setItem('vshell.theme', dark.value ? 'dark' : 'light')
}
</script>

<template>
  <div class="login-page">
    <!-- 左侧品牌区 -->
    <div class="brand-pane">
      <div class="brand-inner">
        <div class="logo-mark">VS</div>
        <h1 class="brand-title">VShell 管理平台</h1>
        <p class="brand-desc">客户端管理 · 监听 · 客户端生成 · 隧道代理 · 插件运行</p>
      </div>
    </div>

    <!-- 右侧表单区 -->
    <div class="form-pane">
      <a class="theme-switch" @click.prevent="toggleTheme">{{ dark ? '亮色主题' : '黑暗主题' }}</a>
      <div class="form-card">
        <h2 class="form-title">管理平台</h2>
        <a-form :model="form" @finish="submit">
          <a-form-item name="username">
            <a-input v-model:value="form.username" size="large" placeholder="请输入账号">
              <template #prefix><UserOutlined /></template>
            </a-input>
          </a-form-item>
          <a-form-item name="password">
            <a-input-password v-model:value="form.password" size="large" placeholder="请输入密码"
              @press-enter="submit">
              <template #prefix><LockOutlined /></template>
            </a-input-password>
          </a-form-item>
          <div class="row-between">
            <a-checkbox v-model:checked="remember">记住我</a-checkbox>
            <a class="forget">忘记密码?</a>
          </div>
          <a-button type="primary" size="large" block :loading="loading" class="submit" @click="submit">
            登录
          </a-button>
        </a-form>
        <div class="foot">基于二进制分析的复刻研究版 · 仅用于授权环境</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page { display: flex; height: 100%; background: var(--el-bg-color, #fff); }
.brand-pane {
  flex: 1 1 50%; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1677ff 0%, #0958d9 55%, #002c8c 100%);
  color: #fff;
}
.brand-inner { max-width: 420px; padding: 0 24px; }
.logo-mark {
  width: 56px; height: 56px; border-radius: 14px; background: rgba(255, 255, 255, 0.18);
  display: flex; align-items: center; justify-content: center; font-size: 22px; font-weight: 700;
  margin-bottom: 20px;
}
.brand-title { font-size: 30px; font-weight: 700; margin: 0 0 12px; }
.brand-desc { font-size: 14px; opacity: 0.85; line-height: 1.8; }
.form-pane { flex: 1 1 50%; position: relative; display: flex; align-items: center; justify-content: center; }
.theme-switch { position: absolute; top: 20px; right: 24px; font-size: 13px; color: #1677ff; }
.form-card { width: 360px; }
.form-title { font-size: 24px; font-weight: 600; margin: 0 0 24px; }
.row-between { display: flex; align-items: center; justify-content: space-between; margin-bottom: 18px; }
.forget { font-size: 13px; }
.submit { margin-top: 4px; }
.foot { margin-top: 24px; font-size: 12px; color: rgba(0, 0, 0, 0.45); text-align: center; }

@media (max-width: 900px) {
  .brand-pane { display: none; }
  .form-pane { flex: 1 1 100%; }
}
</style>
