<template>
  <div class="credit-badge">
    <span class="label">算力余额</span>
    <span class="value">{{ store.userPower }}</span>
    <template v-if="estimatedCost > 0">
      <span class="sep">|</span>
      <span class="label">本次预估</span>
      <span class="cost">-{{ estimatedCost }}</span>
    </template>
  </div>
</template>

<script setup>
import { useEcomConfigStore } from '@/store/ecom'
const store = useEcomConfigStore()
defineProps({ estimatedCost: { type: Number, default: 0 } })
</script>

<style scoped>
.credit-badge {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 13px;
  color: var(--text-color);
}
.label { color: var(--text-secondary); }
.value { color: #fa8c16; font-weight: 700; }
.sep { color: var(--theme-border-primary); }
.cost { color: #f5222d; font-weight: 700; }
</style>
