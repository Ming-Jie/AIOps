import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useRoute, useRouter } from 'vue-router'
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

interface TeamMsgResp {
  id: number
  conversation_id: number
  sender_agent_id: number
  sender_name: string
  content: string
  msg_type: string
  target_agent_id: number | null
  round: number
  metadata: Record<string, unknown> | null
  created_at: string
}

interface TeamConvResponse {
  id: number
  team_id: number
  title: string
  status: string
  started_by: number
  round: number
  messages?: TeamMsgResp[]
  created_at: string
  updated_at: string
}

export function useTeamChatPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const route = useRoute()
  const router = useRouter()

  const team = ref<TeamResponse | null>(null)
  const loading = ref(false)
  const sending = ref(false)
  const errorMsg = ref('')
  const showMembers = ref(false)
  const conversations = ref<TeamConvResponse[]>([])
  const currentConv = ref<TeamConvResponse | null>(null)
  const messages = ref<TeamMsgResp[]>([])
  const inputText = ref('')
  const msgContainer = ref<HTMLElement | null>(null)

  const teamId = computed(() => {
    const raw = String(route.params.teamId ?? '')
    const direct = Number(raw)
    if (Number.isFinite(direct) && direct > 0) return direct
    // 兼容旧书签：/teams/<名称>-<id>/chat
    const legacy = Number(raw.split('-').pop())
    return Number.isFinite(legacy) && legacy > 0 ? legacy : 0
  })

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

  function formatTime (ts: string): string {
    if (!ts) return ''
    const d = new Date(ts)
    return d.toLocaleString()
  }

  async function loadTeam () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<TeamResponse>>(`/teams/${teamId.value}`)
      team.value = (data.data ?? null) as TeamResponse | null
      if (!team.value) {
        errorMsg.value = t('loadFailed')
      }
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? t('loadFailed')
    } finally {
      loading.value = false
    }
  }

  async function loadConversations () {
    try {
      const { data } = await api.get<APIResponse<TeamConvResponse[]>>(`/teams/${teamId.value}/conversations`)
      conversations.value = (data.data ?? []) as TeamConvResponse[]
    } catch {
      // ignore
    }
  }

  async function createConversation () {
    if (!team.value) return
    sending.value = true
    try {
      const { data } = await api.post<APIResponse<TeamConvResponse>>(`/teams/${team.value.id}/conversations?title=${encodeURIComponent(new Date().toLocaleString())}`)
      const conv = (data.data ?? null) as TeamConvResponse | null
      if (conv) {
        conversations.value.unshift(conv)
        selectConversation(conv)
      }
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('createFailed') })
    } finally {
      sending.value = false
    }
  }

  function selectConversation (conv: TeamConvResponse) {
    currentConv.value = conv
    loadMessages(conv.id)
  }

  async function loadMessages (convId: number) {
    try {
      const { data } = await api.get<APIResponse<TeamConvResponse>>(`/teams/conversations/${convId}`)
      const conv = (data.data ?? null) as TeamConvResponse | null
      if (conv) {
        messages.value = conv.messages ?? []
        currentConv.value = conv
        nextTick(() => scrollToBottom())
      }
    } catch {
      // ignore
    }
  }

  async function sendMessage () {
    const text = inputText.value.trim()
    if (!text || !currentConv.value || sending.value) return

    inputText.value = ''
    sending.value = true

    try {
      const { data } = await api.post<APIResponse<TeamConvResponse>>(`/teams/conversations/${currentConv.value.id}/messages`, {
        conversation_id: currentConv.value.id,
        text
      })
      const conv = (data.data ?? null) as TeamConvResponse | null
      if (conv) {
        messages.value = conv.messages ?? []
        currentConv.value = conv
        nextTick(() => scrollToBottom())
      }
      await loadConversations()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('sendFailed') })
    } finally {
      sending.value = false
    }
  }

  function scrollToBottom () {
    if (msgContainer.value) {
      msgContainer.value.scrollTop = msgContainer.value.scrollHeight
    }
  }

  function goBack () {
    router.push({ name: 'teams' })
  }

  onMounted(() => {
    const raw = String(route.params.teamId ?? '')
    const id = teamId.value
    if (id > 0 && raw !== String(id)) {
      router.replace({ name: 'team-chat', params: { teamId: String(id) } })
    }
    loadTeam()
    loadConversations()
  })

  // Auto-create a conversation when entering an empty team
  watch(conversations, (convs) => {
    if (convs.length > 0 && !currentConv.value) {
      selectConversation(convs[0])
    }
  }, { immediate: true })

  return {
    t,
    team,
    errorMsg,
    sending,
    showMembers,
    conversations,
    currentConv,
    messages,
    inputText,
    goBack,
    createConversation,
    selectConversation,
    sendMessage,
    modeLabel,
    modeIcon,
    formatTime,
    msgContainer
  }
}
