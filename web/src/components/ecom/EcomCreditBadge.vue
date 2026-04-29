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
  color: #595959;
}
.label { color: #8c8c8c; }
.value { color: #fa8c16; font-weight: 700; }
.sep { color: #d9d9d9; }
.cost { color: #f5222d; font-weight: 700; }
</style>
