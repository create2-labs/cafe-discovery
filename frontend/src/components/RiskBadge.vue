<template>
  <span :class="badgeClass" class="badge">
    {{ riskLabel }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  riskScore: {
    type: Number,
    required: true
  }
})

const riskLabel = computed(() => {
  if (props.riskScore >= 0.7) return 'High Risk'
  if (props.riskScore >= 0.4) return 'Medium Risk'
  if (props.riskScore >= 0.1) return 'Low Risk'
  return 'Safe'
})

const badgeClass = computed(() => {
  if (props.riskScore >= 0.7) return 'badge-high'
  if (props.riskScore >= 0.4) return 'badge-medium'
  if (props.riskScore >= 0.1) return 'badge-low'
  return 'badge-safe'
})
</script>

