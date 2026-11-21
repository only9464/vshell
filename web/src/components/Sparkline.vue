<script setup lang="ts">
// 轻量折线图：纯 SVG，不引第三方图表库（保持包体小、渲染快）。
// 用途：仪表盘展示 CPU / 内存的近期采样趋势。
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  values: number[]
  max?: number
  height?: number
  width?: number
  color?: string
}>(), { max: 100, height: 48, width: 240, color: '#409eff' })

const points = computed(() => {
  const vs = props.values
  if (vs.length === 0) return ''
  const stepX = vs.length > 1 ? props.width / (vs.length - 1) : 0
  return vs
    .map((v, i) => {
      const clamped = Math.max(0, Math.min(v, props.max))
      const x = i * stepX
      const y = props.height - (clamped / props.max) * props.height
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
})

const area = computed(() => {
  if (!points.value) return ''
  return `0,${props.height} ${points.value} ${props.width},${props.height}`
})

const last = computed(() => props.values[props.values.length - 1] ?? 0)
</script>

<template>
  <div class="spark">
    <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`" role="img">
      <polygon :points="area" :fill="color" opacity="0.12" />
      <polyline :points="points" fill="none" :stroke="color" stroke-width="1.5" />
    </svg>
    <span class="last mono">{{ Math.round(last) }}%</span>
  </div>
</template>

<style scoped>
.spark { display: flex; align-items: center; gap: 8px; }
.last { font-size: 12px; color: var(--el-text-color-secondary); }
</style>
