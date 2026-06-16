import { ref, reactive, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useRouter } from 'vue-router'
import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

interface TeamMemberResp {
  id: number
  team_id: number
  agent_id: number
  agent_name: string
  role: string
  sort_order: number
}

interface TeamResponse {
  id: number
  name: string
  description: string
  mode: string
  coordinator_agent_id: number | null
  max_rounds: number
  is_active: boolean
  config: Record<string, unknown>
  created_by: number
  created_at: string
  updated_at: string
  members?: TeamMemberResp[]
}

interface AgentBrief {
  id: number
  name?: string
  description?: string
}

export function useTeamsPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const router = useRouter()

  const loading = ref(false)
  const saving = ref(false)
  const errorMsg = ref('')
  const teams = ref<TeamResponse[]>([])
  const agents = ref<AgentBrief[]>([])
  const agentMap = computed(() => {
    const map: Record<number, string> = {}
    for (const a of agents.value) {
      map[a.id] = a.name || `Agent(${a.id})`
    }
    return map
  })

  const dialogOpen = ref(false)
  const editingId = ref<number | null>(null)
  const modeInfoDialog = ref(false)

  const debateRounds = ref(3)
  const stanceMap = ref<Record<number, string>>({})

  const form = reactive({
    name: '',
    description: '',
    mode: 'group_chat',
    coordinator_agent_id: null as number | null,
    max_rounds: 5,
    config: {} as Record<string, unknown>,
    agent_ids: [] as number[]
  })

  const agentOptions = computed(() => {
    return agents.value.map(a => ({ label: a.name || a.description || `Agent(${a.id})`, value: a.id }))
  })

  const modeOptions = [
    { label: t('teamModeGroupChat'), value: 'group_chat', description: t('teamModeGroupChatDesc') },
    { label: t('teamModeDebate'), value: 'debate', description: t('teamModeDebateDesc') },
    { label: t('teamModeRouting'), value: 'routing', description: t('teamModeRoutingDesc') },
    { label: t('teamModeSequential'), value: 'sequential', description: t('teamModeSequentialDesc') }
  ]

  const modeDescriptions = computed(() =>
    modeOptions.map(m => ({
      ...m,
      label: m.label,
      desc: m.description,
      icon: modeIcon(m.value)
    }))
  )

  function modeLabel (mode: string): string {
    const map: Record<string, string> = {
      group_chat: t('teamModeGroupChat'),
      debate: t('teamModeDebate'),
      routing: t('teamModeRouting'),
      sequential: t('teamModeSequential')
    }
    return map[mode] || mode
  }

  function modeIcon (mode: string): string {
    const map: Record<string, string> = {
      group_chat: 'groups',
      debate: 'forum',
      routing: 'alt_route',
      sequential: 'linear_scale'
    }
    return map[mode] || 'group_work'
  }

  function getAgentName (agentId: number): string {
    return agentMap.value[agentId] || `Agent(${agentId})`
  }

  async function loadTeams () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<TeamResponse[]>>('/teams')
      teams.value = (data.data ?? []) as TeamResponse[]
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? t('loadFailed')
    } finally {
      loading.value = false
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

  function openDialog (team?: TeamResponse) {
    if (team) {
      editingId.value = team.id
      form.name = team.name
      form.description = team.description
      form.mode = team.mode
      form.coordinator_agent_id = team.coordinator_agent_id
      form.max_rounds = team.max_rounds
      form.config = team.config ?? {}
      form.agent_ids = team.members?.map(m => m.agent_id) ?? []

      if (team.mode === 'debate') {
        debateRounds.value = (team.config?.debate_rounds as number) ?? 3
        stanceMap.value = (team.config?.stances as Record<number, string>) ?? {}
      }
    } else {
      editingId.value = null
      form.name = ''
      form.description = ''
      form.mode = 'group_chat'
      form.coordinator_agent_id = null
      form.max_rounds = 5
      form.config = {}
      form.agent_ids = []
      debateRounds.value = 3
      stanceMap.value = {}
    }
    dialogOpen.value = true
  }

  function buildConfig (): Record<string, unknown> {
    const config: Record<string, unknown> = { ...form.config }
    if (form.mode === 'debate') {
      config.debate_rounds = debateRounds.value
      const stances: Record<string, string> = {}
      for (const [aid, stance] of Object.entries(stanceMap.value)) {
        if (stance) stances[aid] = stance
      }
      if (Object.keys(stances).length > 0) config.stances = stances
    }
    return config
  }

  async function saveTeam () {
    if (!form.name) {
      $q.notify({ type: 'negative', message: t('teamNameRequired') })
      return
    }
    saving.value = true
    try {
      const payload = {
        name: form.name,
        description: form.description,
        mode: form.mode,
        coordinator_agent_id: form.coordinator_agent_id,
        max_rounds: form.max_rounds,
        config: buildConfig(),
        agent_ids: form.agent_ids
      }

      if (editingId.value) {
        await api.put(`/teams/${editingId.value}`, payload)
        $q.notify({ type: 'positive', message: t('updateSuccess') })
      } else {
        await api.post('/teams', payload)
        $q.notify({ type: 'positive', message: t('createSuccess') })
      }
      dialogOpen.value = false
      await loadTeams()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    } finally {
      saving.value = false
    }
  }

  function confirmDelete (team: TeamResponse) {
    $q.dialog({
      title: t('delete'),
      message: t('teamDeleteConfirm', { name: team.name }),
      cancel: true,
      ok: { color: 'negative', label: t('delete') }
    }).onOk(async () => {
      try {
        await api.delete(`/teams/${team.id}`)
        $q.notify({ type: 'positive', message: t('deleteSuccess') })
        await loadTeams()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  function openTeamChat (team: TeamResponse) {
    router.push({ name: 'team-chat', params: { teamId: String(team.id) } })
  }

  async function startConversation (team: TeamResponse) {
    router.push({ name: 'team-chat', params: { teamId: String(team.id) } })
  }

  onMounted(() => {
    loadTeams()
    loadAgents()
  })

  return {
    t,
    loading,
    saving,
    teams,
    errorMsg,
    dialogOpen,
    editingId,
    form,
    modeInfoDialog,
    modeOptions,
    agentOptions,

    debateRounds,
    stanceMap,
    modeDescriptions,
    loadTeams,
    openDialog,
    saveTeam,
    confirmDelete,
    openTeamChat,
    startConversation,
    modeLabel,
    modeIcon,
    getAgentName
  }
}
