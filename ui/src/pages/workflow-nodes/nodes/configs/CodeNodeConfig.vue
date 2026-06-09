<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfCodeLanguage') }}</div>
      <q-select :model-value="strField('language') || 'python'" :options="languageOptions" outlined dense emit-value map-options class="config-input" @update:model-value="patchConfig('language', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfCode') }}</div>
      <div class="text-caption text-grey-7 q-mb-xs">{{ t('wfCodeHint') }}</div>
      <q-input :model-value="strField('code')" outlined dense type="textarea" rows="10" :placeholder="t('wfCodePh')" class="config-input" @update:model-value="patchConfig('code', $event)" />
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

const languageOptions = [
  { label: 'Python', value: 'python' },
  { label: 'JavaScript', value: 'javascript' }
]

function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
}

function strField (key: string): string {
  const v = props.config[key]
  return v == null ? '' : String(v)
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
