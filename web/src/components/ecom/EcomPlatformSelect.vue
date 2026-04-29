<template>
  <el-select :model-value="modelValue" @update:model-value="emit('update:modelValue', $event)" style="width:100%">
    <el-option v-for="p in platforms" :key="p.value" :value="p.value" :label="p.label" />
  </el-select>
</template>

<script setup>
import { useEcomConfigStore } from '@/store/ecom'
const { platforms } = useEcomConfigStore()
defineProps({ modelValue: { type: String, default: 'generic' } })
const emit = defineEmits(['update:modelValue'])
</script>
