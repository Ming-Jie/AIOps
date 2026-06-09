<template>
  <div class="output-vars-display">
    <div v-if="showHint" class="text-caption text-grey-7 q-mb-sm">
      {{ hintText || t('wfOutputVarsHint') }}
    </div>
    <div v-if="variables.length === 0" class="text-caption text-grey">
      {{ emptyText || t('wfOutputVarsEmpty') }}
    </div>
    <div v-else class="output-vars-list">
      <div v-for="v in variables" :key="v.name" class="output-var-row">
        <div class="output-var-main">
          <span class="output-var-name">{{ v.name }}</span>
          <q-badge
            v-if="v.typeHint"
            color="grey-7"
            outline
            dense
            :label="v.typeHint"
            class="q-ml-xs"
          />
        </div>
        <code v-if="nodeId" class="output-var-ref">{{ formatNodeVarRef(nodeId, v.expression) }}</code>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { declaredOutputs, formatNodeVarRef, type VariableSuggestion } from 'src/lib/upstreamOutputs'

const props = withDefaults(defineProps<{
  nodeType?: string
  config?: Record<string, unknown>
  outputSchema?: Record<string, unknown> | null
  nodeId?: string | null
  variables?: VariableSuggestion[]
  showHint?: boolean
  hintText?: string
  emptyText?: string
}>(), {
  nodeType: '',
  config: () => ({}),
  outputSchema: null,
  nodeId: null,
  variables: () => [],
  showHint: true,
  hintText: '',
  emptyText: ''
})

const { t } = useI18n()

const variables = computed((): VariableSuggestion[] => {
  if (props.variables.length > 0) return props.variables
  if (!props.nodeType) return []
  return declaredOutputs(props.nodeType, props.config, props.outputSchema)
})
</script>

<style scoped>
.output-vars-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  background: #fafafa;
}

.output-var-row {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.output-var-main {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
}

.output-var-name {
  font-size: 13px;
  font-weight: 600;
  color: #333;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.output-var-ref {
  font-size: 11px;
  color: #666;
  background: #fff;
  border: 1px solid #eee;
  border-radius: 4px;
  padding: 2px 6px;
  width: fit-content;
}
</style>
