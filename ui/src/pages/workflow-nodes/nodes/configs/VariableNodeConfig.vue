<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfVariableAssignments') }}</div>
      <q-input
        :model-value="jsonField('assignments')"
        outlined
        dense
        type="textarea"
        rows="6"
        :placeholder="t('wfVariableAssignmentsPh')"
        class="config-input"
        @update:model-value="patchJSON('assignments', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits(['update:label', 'update:config'])

const { t } = useI18n()

function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
}

function jsonField (key: string): string {
  const v = props.config[key]
  if (v == null || v === '') return '{}'
  if (typeof v === 'string') return v
  try {
    return JSON.stringify(v, null, 2)
  } catch {
    return String(v)
  }
}

function patchJSON (key: string, value: unknown) {
  const raw = value == null ? '' : String(value).trim()
  if (!raw) {
    patchConfig(key, {})
    return
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    patchConfig(key, parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {})
  } catch {
    patchConfig(key, raw)
  }
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
