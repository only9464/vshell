<script setup lang="ts">
// 轻量折线（纯 SVG），用于工作台的 CPU / 磁盘趋势
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  values: number[]; max?: number; height?: number; width?: number; color?: string
}>(), { max: 100, height: 48, width: 260, color: '#1677ff' })

const points = computed(() => {
  const v = props.values
  if (!v.length) return ''
  const step = v.length > 1 ? props.width / (v.length - 1) : 0
  return v.map((n, i) => {
    const y = props.height - (Math.max(0, Math.min(n, props.max)) / props.max) * props.height
    return `${(i * step).toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
})
const area = computed(() => (points.value ? `0,${props.height} ${points.value} ${props.width},${props.height}` : ''))
const last = computed(() => props.values[props.values.length - 1] ?? 0)
</script>

<template>
  <div class="spark">
    <svg :width="width" :height="height" :viewBox="`0 0 ${width} ${height}`">
      <polygon :points="area" :fill="color" opacity="0.12" />
      <polyline :points="points" fill="none" :stroke="color" stroke-width="1.5" />
    </svg>
    <span class="last mono">{{ Math.round(last) }}%</span>
  </div>
</template>

<style scoped>
.spark { display: flex; align-items: center; gap: 8px; }
.last { font-size: 12px; color: rgba(0, 0, 0, 0.45); }
</style>
