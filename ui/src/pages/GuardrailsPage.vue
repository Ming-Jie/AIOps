<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('guardrails') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createGuardrail')" icon="add" @click="openDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense @click="loadRules" :loading="loadingRules" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <!-- Rule type filter tabs -->
    <q-tabs v-model="tab" class="q-mb-md" dense inline-label>
      <q-tab name="rules" :label="t('guardrailRules')" />
      <q-tab name="logs" :label="t('guardrailLogs')" />
    </q-tabs>

    <q-tab-panels v-model="tab" animated>
      <!-- Rules Panel -->
      <q-tab-panel name="rules" class="q-pa-none">
        <q-table
          flat bordered class="radius-sm" :rows="rules" :columns="ruleColumns" row-key="id"
          :loading="loadingRules" :no-data-label="t('guardrailNoRules')"
          :rows-per-page-options="[10, 20, 50]"
        >
          <template #body-cell-rule_type="props">
            <q-td :props="props">
              <q-chip dense :color="ruleTypeColor(props.row.rule_type)" text-color="white" size="sm">
                {{ ruleTypeLabel(props.row.rule_type) }}
              </q-chip>
            </q-td>
          </template>
          <template #body-cell-action="props">
            <q-td :props="props">
              <q-badge :color="actionColor(props.row.action)">{{ actionLabel(props.row.action) }}</q-badge>
            </q-td>
          </template>
          <template #body-cell-is_active="props">
            <q-td :props="props">
              <q-toggle v-model="props.row.is_active" dense @update:model-value="toggleActive(props.row)" />
            </q-td>
          </template>
          <template #body-cell-hit_count="props">
            <q-td :props="props" class="text-right">
              <span class="text-weight-medium">{{ props.row.hit_count }}</span>
              <span v-if="props.row.last_hit_at" class="text-caption text-grey q-ml-xs">
                · {{ formatTime(props.row.last_hit_at) }}
              </span>
            </q-td>
          </template>
          <template #body-cell-actions="props">
            <q-td :props="props">
              <q-btn dense flat color="primary" :label="t('edit')" @click="openDialog(props.row)" />
              <q-btn dense flat color="secondary" :label="t('guardrailTest')" @click="openTest(props.row)" />
              <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
            </q-td>
          </template>
        </q-table>
      </q-tab-panel>

      <!-- Logs Panel -->
      <q-tab-panel name="logs" class="q-pa-none">
        <div class="row q-mb-md q-gutter-sm">
          <q-select
            v-model="logFilter.rule_type" :label="t('guardrailType')" :options="ruleTypeOptions"
            clearable outlined dense style="min-width:180px" @update:model-value="loadLogs"
          />
          <q-select
            v-model="logFilter.blocked" :label="t('guardrailAction')" :options="logActionOptions"
            clearable outlined dense style="min-width:120px" @update:model-value="loadLogs"
          />
        </div>

        <q-table
          flat bordered class="radius-sm" :rows="logs" :columns="logColumns" row-key="id"
          :loading="loadingLogs" :no-data-label="t('guardrailNoLogs')"
          :pagination="logPagination" @request="onLogPageRequest"
        >
          <template #body-cell-blocked="props">
            <q-td :props="props">
              <q-badge :color="props.row.blocked ? 'negative' : 'positive'">
                {{ props.row.blocked ? t('guardrailBlocked') : t('guardrailPassed') }}
              </q-badge>
            </q-td>
          </template>
          <template #body-cell-rule_type="props">
            <q-td :props="props">
              <q-chip dense size="sm">{{ ruleTypeLabel(props.row.rule_type) }}</q-chip>
            </q-td>
          </template>
          <template #body-cell-input="props">
            <q-td :props="props" class="text-ellipsis" style="max-width:200px">
              <div class="text-caption ellipsis">{{ props.row.input }}</div>
            </q-td>
          </template>
          <template #body-cell-created_at="props">
            <q-td :props="props">
              <span class="text-caption">{{ formatTime(props.row.created_at) }}</span>
            </q-td>
          </template>
        </q-table>
      </q-tab-panel>
    </q-tab-panels>

    <!-- Rule Editor Dialog -->
    <q-dialog v-model="dialogOpen" :maximized="isMaximized" @keydown.escape="dialogOpen = false">
      <q-card style="min-width:600px;max-width:800px">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ editingId ? t('editGuardrail') : t('createGuardrail') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-separator />

        <q-card-section class="scroll" style="max-height:70vh">
          <q-input v-model="form.name" :label="t('guardrailName')" outlined dense class="q-mb-sm" :rules="[v => !!v || t('guardrailNameRequired')]" />
          <q-input v-model="form.description" :label="t('description')" outlined dense class="q-mb-md" />

          <div class="row q-col-gutter-sm">
            <div class="col-6">
              <q-select v-model="form.rule_type" :label="t('guardrailType')" :options="ruleTypeOptions" outlined dense emit-value map-options />
            </div>
            <div class="col-6">
              <q-select v-model="form.action" :label="t('guardrailAction')" :options="actionOptions" outlined dense emit-value map-options />
            </div>
          </div>

          <!-- Rule type description -->
          <q-banner v-if="form.rule_type" class="bg-grey-2 radius-sm q-mb-md q-mt-sm" dense>
            <template #avatar>
              <q-icon name="info" color="primary" />
            </template>
            {{ typeDescription(form.rule_type) }}
          </q-banner>

          <div class="row q-col-gutter-sm">
            <div class="col-4">
              <q-select v-model="form.scope" :label="t('guardrailScope')" :options="scopeOptions" outlined dense emit-value map-options />
            </div>
            <div class="col-4">
              <q-select v-model="form.severity" :label="t('guardrailSeverity')" :options="severityOptions" outlined dense emit-value map-options />
            </div>
            <div class="col-4">
              <q-input v-model="form.priority" label="Priority" outlined dense type="number" min="0" />
            </div>
          </div>

          <!-- Rule type-specific config -->
          <template v-if="form.rule_type === 'pii_detection'">
            <div class="q-mt-md">
              <div class="text-subtitle2 q-mb-sm">{{ t('guardrailConfig') }}</div>
              <q-option-group v-model="piiTypes" :options="piiTypeOptions" type="checkbox" dense inline />
            </div>
          </template>

          <template v-if="form.rule_type === 'topic_guardrail'">
            <div class="row q-col-gutter-md q-mt-md">
              <div class="col-4">
                <q-select v-model="topicMode" :label="t('guardrailTopicMode')" :options="topicModeOptions" outlined dense emit-value map-options />
              </div>
              <div class="col-8">
                <q-input
                  v-model="topicList" :label="t('guardrailKeywords')" outlined dense type="textarea" autogrow
                  :placeholder="t('guardrailKeywordPlaceholder')"
                />
              </div>
            </div>
          </template>

          <template v-if="form.rule_type === 'keyword_filter'">
            <q-input
              v-model="keywordList" :label="t('guardrailKeywords')" outlined dense type="textarea" autogrow
              :placeholder="t('guardrailKeywordPlaceholder')" class="q-mt-md"
            />
          </template>

          <template v-if="form.rule_type === 'regex_match'">
            <q-input
              v-model="regexPattern" :label="t('guardrailPattern')" outlined dense
              :placeholder="t('guardrailPatternPlaceholder')" class="q-mt-md"
            />
            <div class="text-caption text-grey-7 q-mt-xs">{{ t('guardrailPatternHint') }}</div>
          </template>

          <template v-if="form.rule_type === 'content_moderation'">
            <q-input
              v-model="moderationThreshold" :label="t('guardrailThreshold')" outlined dense type="number"
              step="0.1" min="0" max="1" class="q-mt-md"
            />
          </template>

          <!-- Agent binding -->
          <div class="q-mt-lg">
            <div class="text-subtitle2 q-mb-sm">{{ t('guardrailAgents') }}</div>
            <q-select
              v-model="form.agent_ids" :label="t('guardrailSelectAgents')" :options="agentOptions" multiple
              outlined dense chips stack-use-chips use-chips emit-value map-options
            />
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveRule" :loading="saving" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Test Rule Dialog -->
    <q-dialog v-model="testDialog">
      <q-card style="min-width:500px">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ t('guardrailTest') }}: {{ testingRule?.name }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <q-input
            v-model="testText"
            :label="t('guardrailTestPlaceholder')"
            outlined type="textarea" :rows="5"
            :placeholder="testPlaceholder"
          />
          <q-btn color="primary" :label="t('guardrailTest')" @click="runTest" :loading="testing" class="q-mt-md" unelevated />
        </q-card-section>
        <q-card-section v-if="testResult">
          <q-banner :class="testResult.triggered ? 'bg-negative text-white' : 'bg-positive text-white'" dense>
            {{ testResult.triggered ? t('guardrailTestTriggered') : t('guardrailTestNotTriggered') }}
          </q-banner>
          <div v-if="testResult.triggered" class="q-mt-sm">
            <div class="text-caption">Type: {{ testResult.rule_type }}</div>
            <div class="text-caption">Action: {{ testResult.action }}</div>
            <div class="text-caption">Match: {{ JSON.stringify(testResult.match_info) }}</div>
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useQuasar } from 'quasar'
import { useGuardrailsPage } from './useGuardrailsPage'

defineOptions({ name: 'GuardrailsPage' })

const $q = useQuasar()
const isMaximized = computed(() => $q.screen.lt.md)

const {
  t, loadingRules, loadingLogs, saving, testing,
  rules, logs, errorMsg, tab,
  ruleColumns, logColumns, dialogOpen, form, editingId,
  testDialog, testingRule, testText, testResult,
  logFilter, logPagination,
  ruleTypeOptions, scopeOptions, actionOptions, severityOptions,
  piiTypes, topicMode, topicList, keywordList, regexPattern, moderationThreshold,
  agentOptions, piiTypeOptions, topicModeOptions, logActionOptions,
  loadRules, loadLogs, openDialog, saveRule, confirmDelete,
  toggleActive, openTest, runTest, onLogPageRequest,
  ruleTypeColor, ruleTypeLabel, typeDescription, actionColor, actionLabel, formatTime, testPlaceholder
} = useGuardrailsPage()
</script>
