<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfToolName') }}</div>
      <q-select
        :model-value="strField('tool_name')"
        :options="toolOptions"
        :loading="loadingTools"
        outlined
        dense
        emit-value
        map-options
        clearable
        use-input
        hide-selected
        fill-input
        input-debounce="0"
        :placeholder="t('wfToolNamePh')"
        class="config-input"
        @filter="filterTools"
        @update:model-value="patchConfig('tool_name', $event || '')"
        @new-value="createToolValue"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfToolInput') }}</div>
      <q-input :model-value="strField('tool_input')" outlined dense type="textarea" rows="5" :placeholder="t('wfToolInputPh')" class="config-input" @update:model-value="patchConfig('tool_input', $event)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToolNodeNameOptions, type ToolNameOption } from './useToolNodeConfig'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits(['update:label', 'update:config'])

const { t } = useI18n()
const { loading: loadingTools, toolOptions: allToolOptions } = useToolNodeNameOptions()
const toolOptions = ref<ToolNameOption[]>([])

function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
}

function strField (key: string): string {
  const v = props.config[key]
  return v == null ? '' : String(v)
}

function filterTools (val: string, update: (fn: () => void) => void) {
  update(() => {
    const needle = val.toLowerCase()
    toolOptions.value = allToolOptions.value.filter(opt =>
      opt.label.toLowerCase().includes(needle) ||
      opt.value.toLowerCase().includes(needle)
    )
  })
}

function createToolValue (val: string, done: (value: string, mode?: 'add' | 'add-unique' | 'toggle') => void) {
  const v = val.trim()
  if (!v) return
  done(v, 'add-unique')
  patchConfig('tool_name', v)
}

const nodeLabel = computed({
  get: () => props.nodeLabel,
  set: (v) => emit('update:label', v)
})
</script>

<style scoped>
.config-content {
  padding: 12px 16px;
}
.config-group {
  margin-bottom: 16px;
}
.config-label {
  font-size: 13px;
  color: #333;
  margin-bottom: 6px;
  font-weight: 500;
}
.config-input {
  font-size: 13px;
}
</style>
