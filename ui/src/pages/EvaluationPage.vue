<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('eval') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('evalCreateCase')" icon="add" @click="openCaseDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn color="accent" :label="t('evalStartRun')" icon="play_arrow" @click="openRunDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense @click="loadCases(); loadRuns(); loadStats()" />
    </div>

    <!-- Stats Cards -->
    <div class="row q-col-gutter-md q-mb-md" v-if="stats">
      <div class="col-6 col-md-2">
        <q-card flat bordered class="radius-sm">
          <q-card-section class="text-center">
            <div class="text-h5 text-primary text-weight-bold">{{ stats.total_cases }}</div>
            <div class="text-caption text-grey">{{ t('evalStatsCases') }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-6 col-md-2">
        <q-card flat bordered class="radius-sm">
          <q-card-section class="text-center">
            <div class="text-h5 text-accent text-weight-bold">{{ stats.total_runs }}</div>
            <div class="text-caption text-grey">{{ t('evalStatsRuns') }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-6 col-md-2">
        <q-card flat bordered class="radius-sm">
          <q-card-section class="text-center">
            <div class="text-h5 text-positive text-weight-bold">{{ (stats.avg_score).toFixed(1) }}%</div>
            <div class="text-caption text-grey">{{ t('evalStatsAvgScore') }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-6 col-md-2">
        <q-card flat bordered class="radius-sm">
          <q-card-section class="text-center">
            <div class="text-h5 text-orange text-weight-bold">{{ (stats.best_score).toFixed(1) }}%</div>
            <div class="text-caption text-grey">{{ t('evalStatsBestScore') }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-6 col-md-2">
        <q-card flat bordered class="radius-sm">
          <q-card-section class="text-center">
            <div class="text-h5 text-positive text-weight-bold">{{ stats.total_passed }}</div>
            <div class="text-caption text-grey">{{ t('evalStatsPassed') }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-6 col-md-2">
        <q-card flat bordered class="radius-sm">
          <q-card-section class="text-center">
            <div class="text-h5 text-negative text-weight-bold">{{ stats.total_failed }}</div>
            <div class="text-caption text-grey">{{ t('evalStatsFailed') }}</div>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <!-- Tabs -->
    <q-tabs v-model="tab" class="q-mb-md" dense inline-label>
      <q-tab name="cases" :label="t('evalCases')" />
      <q-tab name="runs" :label="t('evalRuns')" />
    </q-tabs>

    <q-tab-panels v-model="tab" animated>
      <!-- Cases Panel -->
      <q-tab-panel name="cases" class="q-pa-none">
        <q-table
          flat bordered class="radius-sm" :rows="cases" row-key="id"
          :loading="loadingCases" :no-data-label="t('evalNoCases')"
          :rows-per-page-options="[10, 20, 50]"
        >
          <template #header>
            <q-tr>
              <q-th>{{ t('evalName') }}</q-th>
              <q-th>{{ t('evalAgent') }}</q-th>
              <q-th>{{ t('evalInputText') }}</q-th>
              <q-th>{{ t('evalExpectedOutput') }}</q-th>
              <q-th>{{ t('evalTags') }}</q-th>
              <q-th>{{ t('createdAt') }}</q-th>
              <q-th class="text-right">{{ t('actions') }}</q-th>
            </q-tr>
          </template>
          <template #body="props">
            <q-tr :props="props">
              <q-td class="text-center"><span class="text-weight-medium">{{ props.row.name }}</span></q-td>
              <q-td class="text-center"><q-badge color="primary">{{ props.row.agent_name || `Agent#${props.row.agent_id}` }}</q-badge></q-td>
              <q-td class="text-center text-ellipsis" style="max-width:200px"><span class="text-caption">{{ props.row.input_text }}</span></q-td>
              <q-td class="text-center text-ellipsis" style="max-width:150px"><span class="text-caption text-grey">{{ props.row.expected_output || '-' }}</span></q-td>
              <q-td class="text-center">
                <q-chip v-for="tag in (props.row.tags ?? [])" :key="tag" dense size="xs" color="grey-3" text-color="grey-8" class="q-mr-xs">{{ tag }}</q-chip>
                <span v-if="!props.row.tags?.length" class="text-grey-5">-</span>
              </q-td>
              <q-td class="text-center"><span class="text-caption">{{ formatTime(props.row.created_at) }}</span></q-td>
              <q-td class="text-right">
                <q-btn dense flat round icon="visibility" size="sm" @click="openCaseDialog(props.row)" />
                <q-btn dense flat round icon="edit" size="sm" @click="openCaseDialog(props.row)" />
                <q-btn dense flat round icon="delete" size="sm" color="negative" @click="confirmDelete(props.row)" />
              </q-td>
            </q-tr>
          </template>
        </q-table>
      </q-tab-panel>

      <!-- Runs Panel -->
      <q-tab-panel name="runs" class="q-pa-none">
        <q-table
          flat bordered class="radius-sm" :rows="runs" row-key="id"
          :loading="loadingRuns" :no-data-label="t('evalNoRuns')"
          :rows-per-page-options="[10, 20, 50]"
        >
          <template #header>
            <q-tr>
              <q-th>{{ t('evalRunName') }}</q-th>
              <q-th>{{ t('evalAgent') }}</q-th>
              <q-th>{{ t('evalRunStatus') }}</q-th>
              <q-th>{{ t('evalRunScore') }}</q-th>
              <q-th>{{ t('evalPassCount') }}</q-th>
              <q-th>{{ t('evalSummary') }}</q-th>
              <q-th>{{ t('evalRunAt') }}</q-th>
            </q-tr>
          </template>
          <template #body="props">
            <q-tr :props="props" class="cursor-pointer" @click="viewRunDetail(props.row)">
              <q-td class="text-center"><span class="text-weight-medium">{{ props.row.name }}</span></q-td>
              <q-td class="text-center"><q-badge color="primary">{{ props.row.agent_name || `Agent#${props.row.agent_id}` }}</q-badge></q-td>
              <q-td class="text-center">
                <q-badge :color="props.row.status === 'completed' ? 'positive' : props.row.status === 'running' ? 'primary' : 'grey'">
                  {{ props.row.status }}
                </q-badge>
              </q-td>
              <q-td class="text-center">
                <q-linear-progress :value="props.row.score / 100" :color="props.row.score >= 80 ? 'positive' : props.row.score >= 50 ? 'warning' : 'negative'" size="20px" class="radius-sm">
                  <div class="absolute-full flex flex-center text-white text-weight-bold text-caption">{{ props.row.score.toFixed(1) }}%</div>
                </q-linear-progress>
              </q-td>
              <q-td class="text-center">
                <span class="text-positive text-weight-medium">{{ props.row.passed }}</span>
                <span class="text-grey">/</span>
                <span class="text-negative text-weight-medium">{{ props.row.failed }}</span>
              </q-td>
              <q-td class="text-center"><span class="text-caption text-grey">{{ props.row.summary }}</span></q-td>
              <q-td class="text-center"><span class="text-caption">{{ formatTime(props.row.created_at) }}</span></q-td>
            </q-tr>
          </template>
        </q-table>
      </q-tab-panel>
    </q-tab-panels>

    <!-- Case Dialog -->
    <q-dialog v-model="caseDialog" :maximized="isMaximized">
      <q-card style="min-width:600px;max-width:800px">
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">{{ editingId ? t('evalEditCase') : t('evalCreateCase') }}</div>
          <q-space />
          <q-btn flat round dense icon="close" v-close-popup />
        </q-card-section>
        <q-separator />
        <q-card-section class="scroll" style="max-height:65vh">
          <q-form @submit.prevent="saveCase">
            <q-input v-model="form.name" :label="t('evalName')" :rules="[v => !!v || t('evalCaseNameRequired')]" outlined dense class="q-mb-md" />
            <q-input v-model="form.description" :label="t('evalDescription')" outlined dense class="q-mb-md" />
            <q-select
              v-model="form.agent_id" :label="t('evalAgent')" :options="agentOptions"
              outlined dense class="q-mb-md" emit-value map-options
              :rules="[v => !!v || t('evalNoAgentSelected')]"
            />
            <q-input
              v-model="form.input_text" :label="t('evalInputText')" outlined dense type="textarea" :rows="3" class="q-mb-md"
              :rules="[v => !!v || t('evalCaseInputRequired')]"
            />
            <q-input v-model="form.expected_output" :label="t('evalExpectedOutput')" outlined dense type="textarea" :rows="3" class="q-mb-md" />
            <q-select
              v-model="form.tags" :label="t('evalTags')" use-input use-chips multiple input-debounce="0"
              @new-value="addTag" outlined dense class="q-mb-md"
            />
            <div class="row justify-end q-mt-md">
              <q-btn flat :label="t('cancel')" v-close-popup class="q-mr-sm" />
              <q-btn color="primary" :label="t('save')" type="submit" :loading="saving" unelevated />
            </div>
          </q-form>
        </q-card-section>
      </q-card>
    </q-dialog>

    <!-- Run Dialog -->
    <q-dialog v-model="runDialog">
      <q-card style="min-width:450px" class="radius-sm">
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">{{ t('evalStartRun') }}</div>
          <q-space />
          <q-btn flat round dense icon="close" v-close-popup />
        </q-card-section>
        <q-separator />
        <q-card-section>
          <q-input v-model="runForm.name" :label="t('evalRunName')" outlined dense class="q-mb-md" />
          <q-select
            v-model="runForm.agent_id" :label="t('evalAgent')" :options="agentOptions"
            outlined dense class="q-mb-md" emit-value map-options
            :rules="[v => !!v || t('evalNoAgentSelected')]"
          />
          <q-select
            v-model="runForm.case_ids" :label="t('evalSelectCases')" :options="caseOptions"
            outlined dense multiple emit-value map-options class="q-mb-md" clearable
            :hint="t('evalSelectCaseHint')"
          />
        </q-card-section>
        <q-card-section class="text-caption text-grey q-pt-none">
          {{ t('evalRunConfirm') }}
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('evalStartRun')" icon="play_arrow" :loading="running" @click="startRun" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Run Detail Dialog -->
    <q-dialog v-model="runDetailDialog" :maximized="isMaximized">
      <q-card style="min-width:700px;max-width:900px">
        <q-card-section class="row items-center q-pb-none">
          <div class="text-h6">{{ selectedRun?.name || t('evalResults') }}</div>
          <q-space />
          <q-btn flat round dense icon="close" v-close-popup />
        </q-card-section>
        <q-separator />
        <q-card-section v-if="selectedRun" class="scroll" style="max-height:65vh">
          <div class="row q-col-gutter-md q-mb-md">
            <div class="col-12 col-md-3">
              <q-card flat bordered class="radius-sm">
                <q-card-section class="text-center">
                  <div class="text-h5 text-positive text-weight-bold">{{ (selectedRun.score).toFixed(1) }}%</div>
                  <div class="text-caption text-grey">{{ t('evalPassRate') }}</div>
                </q-card-section>
              </q-card>
            </div>
            <div class="col-12 col-md-9">
              <q-card flat bordered class="radius-sm q-pa-md">
                <div class="row q-col-gutter-md">
                  <div class="col-4"><span class="text-grey">{{ t('evalAgent') }}:</span> <strong>{{ selectedRun.agent_name }}</strong></div>
                  <div class="col-4"><span class="text-grey">{{ t('evalPassCount') }}:</span> <strong class="text-positive">{{ selectedRun.passed }}</strong><span class="text-grey">/</span><strong class="text-negative">{{ selectedRun.failed }}</strong></div>
                  <div class="col-4"><span class="text-grey">Total:</span> <strong>{{ selectedRun.total }}</strong></div>
                </div>
                <div class="row q-col-gutter-md q-mt-sm">
                  <div class="col-6"><span class="text-grey">{{ t('evalRunAt') }}:</span> {{ formatTime(selectedRun.created_at) }}</div>
                  <div class="col-6"><span class="text-grey">{{ t('evalSummary') }}:</span> {{ selectedRun.summary }}</div>
                </div>
              </q-card>
            </div>
          </div>

          <q-markup-table flat bordered v-if="selectedRun.results?.length">
            <thead>
              <q-tr>
                <q-th class="text-left">#</q-th>
                <q-th class="text-left">{{ t('evalName') }}</q-th>
                <q-th class="text-left">{{ t('evalPassed') }}</q-th>
                <q-th class="text-left">{{ t('evalScore') }}</q-th>
                <q-th class="text-left">{{ t('evalSimilarity') }}</q-th>
                <q-th class="text-left">{{ t('evalDuration') }}</q-th>
                <q-th class="text-left">{{ t('evalReason') }}</q-th>
                <q-th class="text-left">{{ t('actions') }}</q-th>
              </q-tr>
            </thead>
            <tbody>
              <q-tr v-for="(r, i) in selectedRun.results" :key="r.id">
                <q-td>{{ i + 1 }}</q-td>
                <q-td><span class="text-weight-medium">{{ r.case_name }}</span></q-td>
                <q-td>
                  <q-icon :name="r.passed ? 'check_circle' : 'cancel'" :color="r.passed ? 'positive' : 'negative'" size="24px" />
                </q-td>
                <q-td>
                  <q-linear-progress :value="r.score" :color="r.score >= 0.8 ? 'positive' : r.score >= 0.5 ? 'warning' : 'negative'" size="16px" class="radius-sm" style="min-width:80px">
                    <div class="absolute-full flex flex-center text-white text-caption text-weight-bold">{{ (r.score * 100).toFixed(0) }}%</div>
                  </q-linear-progress>
                </q-td>
                <q-td>
                  <q-badge :color="r.score >= 0.8 ? 'positive' : r.score >= 0.5 ? 'warning' : 'negative'">
                    {{ (r.score * 100).toFixed(0) }}%
                  </q-badge>
                </q-td>
                <q-td class="text-caption">{{ r.duration_ms }}ms</q-td>
                <q-td class="text-caption" style="max-width:200px">
                  <span class="text-ellipsis">{{ r.reason }}</span>
                </q-td>
                <q-td>
                  <q-btn dense flat round icon="visibility" size="sm" @click="showResultDetail = showResultDetail === r.id ? null : r.id" />
                </q-td>
              </q-tr>
              <q-tr v-for="r in selectedRun.results" :key="'detail-' + r.id" v-show="showResultDetail === r.id">
                <q-td colspan="8" class="bg-grey-1">
                  <div class="row q-col-gutter-md">
                    <div class="col-6">
                      <div class="text-caption text-grey q-mb-xs">{{ t('evalInputText') }}</div>
                      <div class="bg-white q-pa-sm radius-sm border">{{ r.input_text }}</div>
                    </div>
                    <div class="col-6">
                      <div class="text-caption text-grey q-mb-xs">{{ t('evalExpectedOutput') }}</div>
                      <div class="bg-white q-pa-sm radius-sm border">{{ r.expected_output || '-' }}</div>
                    </div>
                    <div class="col-12">
                      <div class="text-caption text-grey q-mb-xs">{{ t('evalActualOutput') }}</div>
                      <div class="bg-white q-pa-sm radius-sm border">{{ r.actual_output }}</div>
                    </div>
                  </div>
                </q-td>
              </q-tr>
            </tbody>
          </q-markup-table>
          <div v-else class="text-center text-grey q-py-lg">
            <q-icon name="info" size="32px" />
            <div class="q-mt-sm">{{ t('noData') }}</div>
          </div>
        </q-card-section>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useQuasar } from 'quasar'
import { useEvalPage } from 'pages/useEvalPage'

const $q = useQuasar()
const isMaximized = computed(() => $q.screen.lt.md)

const {
  t, loadingCases, loadingRuns, saving, running,
  cases, runs, stats, tab,
  caseDialog, editingId, form, agentOptions,
  runDialog, runForm, caseOptions,
  runDetailDialog, selectedRun,
  formatTime,
  loadCases, loadRuns, loadStats,
  openCaseDialog, saveCase, confirmDelete,
  openRunDialog, startRun, viewRunDetail
} = useEvalPage()

const showResultDetail = ref<number | null>(null)

function addTag (val: string, done: any) {
  if (val) {
    done(val)
  }
  done(null)
}
</script>
