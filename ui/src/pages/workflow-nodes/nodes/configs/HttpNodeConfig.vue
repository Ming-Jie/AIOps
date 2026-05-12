<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfHTTPMethod') }}</div>
      <q-select
        :model-value="strField('method') || 'GET'"
        :options="methodOptions"
        outlined
        dense
        emit-value
        map-options
        class="config-input"
        @update:model-value="patchConfig('method', $event)"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfHTTPURL') }}</div>
      <q-input :model-value="strField('url')" outlined dense :placeholder="t('wfHTTPURLPh')" class="config-input" @update:model-value="patchConfig('url', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfHTTPHeaders') }}</div>
      <q-input :model-value="jsonField('headers')" outlined dense type="textarea" rows="4" :placeholder="t('wfHTTPHeadersPh')" class="config-input" @update:model-value="patchJSON('headers', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfHTTPBody') }}</div>
      <q-input :model-value="strField('body')" outlined dense type="textarea" rows="5" :placeholder="t('wfHTTPBodyPh')" class="config-input" @update:model-value="patchConfig('body', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfHTTPTimeout') }}</div>
      <q-input :model-value="numField('timeout_ms') || 30000" type="number" outlined dense min="1000" max="300000" step="1000" class="config-input" @update:model-value="patchNumber('timeout_ms', $event)" />
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

const methodOptions = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'].map(v => ({ label: v, value: v }))

function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
}

function strField (key: string): string {
  const v = props.config[key]
  return v == null ? '' : String(v)
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

function numField (key: string): number {
  const v = props.config[key]
  if (v == null || v === '') return 0
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isNaN(n) ? 0 : n
}

function patchNumber (key: string, raw: string | number | null | undefined) {
  if (raw === '' || raw === null || raw === undefined) {
    patchConfig(key, 0)
    return
  }
  const n = typeof raw === 'number' ? raw : Number(raw)
  patchConfig(key, Number.isNaN(n) ? 0 : n)
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
