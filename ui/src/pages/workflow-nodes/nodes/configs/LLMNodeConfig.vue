<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfBindAgent') }}</div>
      <q-select
        :model-value="bindAgentId"
        :options="agentOptions"
        :loading="agentsLoading"
        outlined
        dense
        emit-value
        map-options
        clearable
        :placeholder="t('wfSelectAgentPh')"
        class="config-input"
        @update:model-value="onAgentId"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfPromptTemplate') }}</div>
      <q-input
        :model-value="strField('prompt')"
        outlined
        dense
        type="textarea"
        rows="6"
        :placeholder="t('wfPromptTemplatePh')"
        class="config-input"
        @update:model-value="patchConfig('prompt', $event)"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfLLMSystemPrompt') }}</div>
      <q-input :model-value="strField('system_prompt')" outlined dense type="textarea" rows="3" :placeholder="t('wfLLMSystemPromptPh')" class="config-input" @update:model-value="patchConfig('system_prompt', $event)" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfLLMTemperature') }}</div>
      <q-input :model-value="numField('temperature') || 0.7" type="number" outlined dense min="0" max="2" step="0.1" class="config-input" @update:model-value="patchNumber('temperature', $event)" />
    </div>
    <div class="text-caption text-grey-6 q-mt-xs">{{ t('wfLLMUsesServerModel') }}</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useAgentNodeAgentOptions, useAgentNodeForm } from './useAgentNodeConfig'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits(['update:label', 'update:config'])

const { t } = useI18n()
const { agentOptions, loading: agentsLoading } = useAgentNodeAgentOptions()

const {
  bindAgentId,
  patchConfig,
  onAgentId,
  strField,
  numField,
  patchNumber,
  nodeLabel
} = useAgentNodeForm(props, emit)
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
