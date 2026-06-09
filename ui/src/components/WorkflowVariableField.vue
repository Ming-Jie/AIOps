<template>
  <VariableExprField
    :model-value="modelValue"
    :placeholder="placeholder"
    :rows="rows"
    :disabled="disabled"
    :show-picker="showPicker"
    :show-toolbar="showToolbar"
    :graph-nodes="graphNodes"
    :graph-edges="graphEdges"
    :selected-node-id="selectedNodeId"
    :latest-debug="latestDebug"
    @update:model-value="$emit('update:modelValue', $event)"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import VariableExprField from 'components/VariableExprField.vue'
import { useWorkflowVariableContext } from 'src/composables/useWorkflowVariableContext'

withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  rows?: number
  disabled?: boolean
  showPicker?: boolean
  showToolbar?: boolean
}>(), {
  placeholder: '',
  rows: 3,
  showPicker: true,
  showToolbar: true
})

defineEmits<{
  'update:modelValue': [value: string]
}>()

const ctx = useWorkflowVariableContext()

const graphNodes = computed(() => ctx?.nodes.value ?? [])
const graphEdges = computed(() => ctx?.edges.value ?? [])
const selectedNodeId = computed(() => ctx?.selectedNodeId.value ?? null)
const latestDebug = computed(() => ctx?.latestDebug.value ?? {})
</script>
