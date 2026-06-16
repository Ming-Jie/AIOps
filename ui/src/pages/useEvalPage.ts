import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

interface EvalCase {
  id: number
  name: string
  description: string
  agent_id: number
  agent_name: string
  input_text: string
  expected_output: string
  criteria: Record<string, unknown>
  tags: string[]
  is_active: boolean
  created_by: number
  created_at: string
  updated_at: string
}

interface EvalResult {
  id: number
  run_id: number
  case_id: number
  case_name: string
  input_text: string
  expected_output: string
  actual_output: string
  passed: boolean
  score: number
  reason: string
  duration_ms: number
  metadata: Record<string, unknown>
}

interface EvalRun {
  id: number
  name: string
  agent_id: number
  agent_name: string
  status: string
  total: number
  passed: number
  failed: number
  score: number
  summary: string
  results?: EvalResult[]
  started_at?: string
  ended_at?: string
  created_at: string
}

interface EvalStats {
  total_cases: number
  total_runs: number
  avg_score: number
  best_score: number
  total_passed: number
  total_failed: number
  recent_runs?: EvalRun[]
}

interface AgentBrief {
  id: number
  name?: string
}

export function useEvalPage () {
  const { t } = useI18n()
  const $q = useQuasar()

  const loadingCases = ref(false)
  const loadingRuns = ref(false)
  const loadingStats = ref(false)
  const saving = ref(false)
  const running = ref(false)
  const tab = ref('cases')

  const cases = ref<EvalCase[]>([])
  const runs = ref<EvalRun[]>([])
  const stats = ref<EvalStats | null>(null)
  const agents = ref<AgentBrief[]>([])

  const caseDialog = ref(false)
  const editingId = ref<number | null>(null)

  const runDialog = ref(false)
  const runDetailDialog = ref(false)
  const selectedRun = ref<EvalRun | null>(null)

  const form = reactive({
    name: '',
    description: '',
    agent_id: null as number | null,
    input_text: '',
    expected_output: '',
    tags: [] as string[]
  })

  const runForm = reactive({
    name: '',
    agent_id: null as number | null,
    case_ids: [] as number[]
  })

  const agentOptions = computed(() => {
    return agents.value.map(a => ({ label: a.name || `Agent(${a.id})`, value: a.id }))
  })

  const caseOptions = computed(() => {
    return cases.value.filter(c => {
      if (!runForm.agent_id) return true
      return c.agent_id === runForm.agent_id
    }).map(c => ({ label: c.name, value: c.id }))
  })

  const runStatusClass = (status: string) => {
    if (status === 'completed') return 'text-positive'
    if (status === 'running') return 'text-primary'
    if (status === 'failed') return 'text-negative'
    return 'text-grey'
  }

  function formatTime (ts: string): string {
    if (!ts) return ''
    const d = new Date(ts)
    return d.toLocaleString()
  }

  async function loadCases () {
    loadingCases.value = true
    try {
      const { data } = await api.get<APIResponse<EvalCase[]>>('/eval/cases')
      cases.value = (data.data ?? []) as EvalCase[]
    } catch {
      $q.notify({ type: 'negative', message: t('evalLoadFailed') })
    } finally {
      loadingCases.value = false
    }
  }

  async function loadRuns () {
    loadingRuns.value = true
    try {
      const { data } = await api.get<APIResponse<EvalRun[]>>('/eval/runs')
      runs.value = (data.data ?? []) as EvalRun[]
    } catch {
      $q.notify({ type: 'negative', message: t('evalLoadFailed') })
    } finally {
      loadingRuns.value = false
    }
  }

  async function loadStats () {
    loadingStats.value = true
    try {
      const { data } = await api.get<APIResponse<EvalStats>>('/eval/stats')
      stats.value = (data.data ?? null) as EvalStats | null
    } catch {
      // non-critical
    } finally {
      loadingStats.value = false
    }
  }

  async function loadAgents () {
    try {
      const { data } = await api.get<APIResponse<AgentBrief[]>>('/agents/all')
      agents.value = (data.data ?? []) as AgentBrief[]
    } catch {
      // non-critical
    }
  }

  function openCaseDialog (c?: EvalCase) {
    if (c) {
      editingId.value = c.id
      form.name = c.name
      form.description = c.description
      form.agent_id = c.agent_id
      form.input_text = c.input_text
      form.expected_output = c.expected_output
      form.tags = c.tags ?? []
    } else {
      editingId.value = null
      form.name = ''
      form.description = ''
      form.agent_id = null
      form.input_text = ''
      form.expected_output = ''
      form.tags = []
    }
    caseDialog.value = true
  }

  async function saveCase () {
    if (!form.name) {
      $q.notify({ type: 'negative', message: t('evalCaseNameRequired') })
      return
    }
    if (!form.input_text) {
      $q.notify({ type: 'negative', message: t('evalCaseInputRequired') })
      return
    }
    saving.value = true
    try {
      const payload = {
        name: form.name,
        description: form.description,
        agent_id: form.agent_id,
        input_text: form.input_text,
        expected_output: form.expected_output,
        tags: form.tags
      }
      if (editingId.value) {
        await api.put(`/eval/cases/${editingId.value}`, payload)
        $q.notify({ type: 'positive', message: t('updateSuccess') })
      } else {
        await api.post('/eval/cases', payload)
        $q.notify({ type: 'positive', message: t('createSuccess') })
      }
      caseDialog.value = false
      await loadCases()
      await loadStats()
    } catch {
      $q.notify({ type: 'negative', message: t('evalSaveFailed') })
    } finally {
      saving.value = false
    }
  }

  function confirmDelete (c: EvalCase) {
    $q.dialog({
      title: t('delete'),
      message: t('evalDeleteCaseConfirm', { name: c.name }),
      cancel: true,
      ok: { color: 'negative', label: t('delete') }
    }).onOk(async () => {
      try {
        await api.delete(`/eval/cases/${c.id}`)
        $q.notify({ type: 'positive', message: t('evalDeleteSuccess') })
        await loadCases()
        await loadStats()
      } catch {
        $q.notify({ type: 'negative', message: t('evalDeleteFailed') })
      }
    })
  }

  function openRunDialog () {
    runForm.name = ''
    runForm.agent_id = null
    runForm.case_ids = []
    runDialog.value = true
  }

  async function startRun () {
    if (!runForm.agent_id) {
      $q.notify({ type: 'negative', message: t('evalNoAgentSelected') })
      return
    }
    running.value = true
    try {
      const payload: Record<string, unknown> = {
        agent_id: runForm.agent_id,
        name: runForm.name || undefined
      }
      if (runForm.case_ids.length > 0) {
        payload.case_ids = runForm.case_ids
      }
      await api.post('/eval/runs', payload)
      $q.notify({ type: 'positive', message: t('evalRunSuccess') })
      runDialog.value = false
      await loadRuns()
      await loadStats()
    } catch {
      $q.notify({ type: 'negative', message: t('evalSaveFailed') })
    } finally {
      running.value = false
    }
  }

  async function viewRunDetail (run: EvalRun) {
    try {
      const { data } = await api.get<APIResponse<EvalRun>>(`/eval/runs/${run.id}`)
      selectedRun.value = (data.data ?? null) as EvalRun | null
      runDetailDialog.value = true
    } catch {
      $q.notify({ type: 'negative', message: t('evalLoadFailed') })
    }
  }

  onMounted(() => {
    loadCases()
    loadRuns()
    loadStats()
    loadAgents()
  })

  return {
    t,
    loadingCases,
    loadingRuns,
    loadingStats,
    saving,
    running,
    cases,
    runs,
    stats,
    agents,
    tab,
    caseDialog,
    editingId,
    form,
    agentOptions,
    runDialog,
    runForm,
    caseOptions,
    runDetailDialog,
    selectedRun,
    formatTime,
    runStatusClass,
    loadCases,
    loadRuns,
    loadStats,
    openCaseDialog,
    saveCase,
    confirmDelete,
    openRunDialog,
    startRun,
    viewRunDetail
  }
}
