<template>
  <q-dialog :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)">
    <q-card class="wf-checklist-dialog">
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">{{ t('wfChecklistTitle') }}</div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-card-section>
        <div v-if="issues.length === 0" class="row items-center text-positive">
          <q-icon name="check_circle" class="q-mr-sm" />
          {{ t('wfChecklistAllClear') }}
        </div>

        <q-list v-else bordered separator class="rounded-borders">
          <q-item
            v-for="issue in issues"
            :key="issue.id"
            clickable
            :class="issue.nodeId ? 'cursor-pointer' : ''"
            @click="onIssueClick(issue)"
          >
            <q-item-section avatar>
              <q-icon
                :name="issue.level === 'error' ? 'error' : 'warning'"
                :color="issue.level === 'error' ? 'negative' : 'warning'"
              />
            </q-item-section>
            <q-item-section>
              <q-item-label>{{ issue.message }}</q-item-label>
              <q-item-label v-if="issue.nodeId" caption>{{ issue.nodeId }}</q-item-label>
            </q-item-section>
          </q-item>
        </q-list>

        <div v-if="issues.length > 0" class="text-caption text-grey-7 q-mt-sm">
          {{ t('wfChecklistHint') }}
        </div>
      </q-card-section>

      <q-card-actions align="right">
        <q-btn flat :label="t('close')" v-close-popup />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ChecklistIssue } from 'src/lib/workflowChecklist'

defineProps<{
  modelValue: boolean
  issues: ChecklistIssue[]
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
  'focus-node': [nodeId: string]
}>()

const { t } = useI18n()

function onIssueClick (issue: ChecklistIssue) {
  if (!issue.nodeId) return
  emit('focus-node', issue.nodeId)
}
</script>

<style scoped>
.wf-checklist-dialog {
  min-width: 480px;
  max-width: 640px;
}
</style>
