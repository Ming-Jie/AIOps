<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfBindKb') }}</div>
      <div class="text-caption text-grey-7 q-mb-xs">{{ t('wfBindKbHint') }}</div>
      <KnowledgeBaseSelect
        :model-value="bindKbId"
        @update:model-value="onKbSelect"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfKnowledgeQuery') }}</div>
      <WorkflowVariableField
        :model-value="strField('query')"
        :rows="2"
        :placeholder="t('wfKnowledgeQueryPh')"
        @update:model-value="patchConfig('query', $event)"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfKnowledgeTopK') }}</div>
      <q-input :model-value="numField('top_k')" type="number" outlined dense class="config-input" min="1" max="50" @update:model-value="patchNumber('top_k', $event)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import KnowledgeBaseSelect from 'components/KnowledgeBaseSelect.vue'
import WorkflowVariableField from 'components/WorkflowVariableField.vue'
import { useKnowledgeNodeForm } from './useKnowledgeNodeConfig'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits(['update:label', 'update:config'])

const { t } = useI18n()

const {
  bindKbId,
  patchConfig,
  patchNumber,
  onKbSelect,
  strField,
  numField,
  nodeLabel
} = useKnowledgeNodeForm(props, emit)
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
