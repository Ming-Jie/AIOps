<template>
  <div class="wf-next-steps">
    <div v-if="steps.length === 0" class="wf-next-steps-empty">
      <q-icon name="link_off" size="16px" class="q-mr-xs" />
      {{ t('wfNextStepNoOutputs') }}
    </div>

    <div v-else class="wf-next-steps-list">
      <div v-for="step in steps" :key="step.output.name" class="wf-next-step-card">
        <div class="wf-next-step-header">
          <div class="min-w-0">
            <div class="wf-next-step-output">{{ step.output.name }}</div>
            <div v-if="step.output.typeHint" class="wf-next-step-type">{{ step.output.typeHint }}</div>
          </div>
          <q-badge
            :color="step.targets.length > 0 ? 'primary' : 'grey-5'"
            :outline="step.targets.length === 0"
            :label="step.targets.length > 0 ? t('wfNextStepConnected') : t('wfNextStepDisconnected')"
          />
        </div>

        <div v-if="step.targets.length > 0" class="wf-next-step-targets">
          <div
            v-for="target in step.targets"
            :key="target.id"
            class="wf-next-step-target"
            @click="$emit('focus-node', target.id)"
          >
            <div class="wf-next-step-target-icon" :style="{ backgroundColor: nodeColor(target.nodeType) }">
              <q-icon :name="nodeIcon(target.nodeType)" size="14px" color="white" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="wf-next-step-target-label ellipsis">{{ target.label }}</div>
              <div class="wf-next-step-target-id ellipsis">{{ target.id }}</div>
            </div>
            <q-icon name="arrow_forward" size="16px" color="grey-6" />
          </div>
        </div>

        <div v-else class="wf-next-step-empty-hint">
          <q-icon name="link_off" size="14px" class="q-mr-xs" />
          {{ t('wfNextStepEmptyHint') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { NextStep } from 'src/lib/workflowNextSteps'
import { getNodeHeaderColor, getNodeIcon } from 'src/lib/upstreamOutputs'

defineProps<{
  steps: NextStep[]
}>()

defineEmits<{
  'focus-node': [nodeId: string]
}>()

const { t } = useI18n()

function nodeIcon (nodeType: string): string {
  return getNodeIcon(nodeType)
}

function nodeColor (nodeType: string): string {
  return getNodeHeaderColor(nodeType)
}
</script>

<style scoped>
.wf-next-steps-empty,
.wf-next-step-empty-hint {
  display: flex;
  align-items: flex-start;
  padding: 10px 12px;
  border: 1px dashed #d9d9d9;
  border-radius: 8px;
  background: #fff;
  font-size: 12px;
  color: #666;
}

.wf-next-steps-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.wf-next-step-card {
  border: 1px solid #e8e8e8;
  border-radius: 10px;
  background: #fff;
  padding: 10px 12px;
}

.wf-next-step-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 8px;
}

.wf-next-step-output {
  font-size: 13px;
  font-weight: 600;
  color: #333;
}

.wf-next-step-type {
  margin-top: 2px;
  font-size: 10px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: #888;
}

.wf-next-step-targets {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.wf-next-step-target {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: #f5f5f5;
  cursor: pointer;
  transition: background 0.15s ease;
}

.wf-next-step-target:hover {
  background: #eef5ff;
}

.wf-next-step-target-icon {
  width: 28px;
  height: 28px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.wf-next-step-target-label {
  font-size: 13px;
  font-weight: 500;
  color: #333;
}

.wf-next-step-target-id {
  margin-top: 2px;
  font-size: 10px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  color: #888;
}
</style>
