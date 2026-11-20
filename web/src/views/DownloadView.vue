<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { listenerApi, type ListenerItem } from '../api'
import { downloadURL } from '../api/http'

const listeners = ref<ListenerItem[]>([])
const form = reactive({ id: 0, arch: 'windows_amd64', tp: 'tcp', host: '127.0.0.1', port: 4444,
  vkey: '', salt: '', upx: false, proxy: '' })

const arches = ['windows_amd64', 'windows_386', 'linux_amd64', 'linux_arm64', 'darwin_amd64']

async function load() {
  const res = await listenerApi.list()
  listeners.value = res.items || []
  if (!form.id && listeners.value.length) form.id = listeners.value[0].Id
}
onMounted(load)

function download(kind: 'stageless' | 'stage' | 'listen' | 'shellcode' | 'dll' | 'listendll') {
  if (!form.id) return ElMessage.warning('先选择监听器')
  let url = ''
  switch (kind) {
    case 'stageless':
      url = downloadURL('/download/stageless', { id: form.id, arch: form.arch, upx: String(form.upx), proxy: form.proxy })
      break
    case 'stage':
      url = downloadURL('/download/stage', { id: form.id, arch: form.arch })
      break
    case 'shellcode':
      url = downloadURL('/download/shellcode', { id: form.id, arch: form.arch })
      break
    case 'listen':
      url = downloadURL('/download/listen', { arch: form.arch, tp: form.tp, host: form.host,
        port: form.port, vkey: form.vkey, salt: form.salt, upx: String(form.upx) })
      break
    case 'dll':
      url = downloadURL('/download/dll', { id: form.id, arch: form.arch, upx: String(form.upx), proxy: form.proxy })
      break
    case 'listendll':
      url = downloadURL('/download/listendll', { arch: form.arch, tp: form.tp, host: form.host,
        port: form.port, vkey: form.vkey, salt: form.salt })
      break
  }
  // 浏览器导航下载（服务端返回裸二进制 + Content-Disposition）
  location.href = url
}
</script>

<template>
  <div class="page">
    <el-card>
      <el-form inline label-width="90px">
        <el-form-item label="监听器">
          <el-select v-model="form.id" style="width: 180px">
            <el-option v-for="l in listeners" :key="l.Id" :label="`#${l.Id} ${l.Mode} ${l.ListenAddr}`" :value="l.Id" />
          </el-select>
        </el-form-item>
        <el-form-item label="架构">
          <el-select v-model="form.arch" style="width: 170px">
            <el-option v-for="a in arches" :key="a" :label="a" :value="a" />
          </el-select>
        </el-form-item>
        <el-form-item label="传输">
          <el-select v-model="form.tp" style="width: 110px">
            <el-option v-for="t in ['tcp','kcp','ws','http']" :key="t" :label="t" :value="t" />
          </el-select>
        </el-form-item>
        <el-form-item label="回连地址"><el-input v-model="form.host" style="width: 140px" /></el-form-item>
        <el-form-item label="端口"><el-input-number v-model="form.port" :min="1" :max="65535" /></el-form-item>
        <el-form-item label="Vkey"><el-input v-model="form.vkey" style="width: 160px" /></el-form-item>
        <el-form-item label="加密盐"><el-input v-model="form.salt" style="width: 160px" /></el-form-item>
        <el-form-item label="UPX"><el-switch v-model="form.upx" /></el-form-item>
      </el-form>

      <el-divider />
      <div class="toolbar" style="flex-wrap: wrap">
        <el-button type="primary" @click="download('stageless')">Stageless（按监听器）</el-button>
        <el-button @click="download('stage')">Stage</el-button>
        <el-button @click="download('shellcode')">Shellcode</el-button>
        <el-button @click="download('dll')">DLL</el-button>
        <el-button @click="download('listen')">Listen（独立）</el-button>
        <el-button @click="download('listendll')">ListenDLL</el-button>
      </div>
      <el-alert type="info" :closable="false" style="margin-top: 12px"
        title="服务端从 stageless|dll/<tp>_<arch> 读取模板；模板缺失会返回 open …: file does not exist。" />
    </el-card>
  </div>
</template>
