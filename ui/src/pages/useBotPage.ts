import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

export type BotType = 'lark' | 'telegram' | 'dingtalk' | 'qq'

/** 智能机器人列表行 */
export interface BotRow {
  app_id: string
  agent_id: number
  agent_name: string
  is_running: boolean
  bot_type?: BotType
  chat_id?: string
  token_prefix?: string
}

export interface BotListResponse {
  bots: BotRow[]
  running: boolean
  bot_count: number
}

function botTypeColor (type?: BotType): string {
  switch (type) {
    case 'lark': return 'blue'
    case 'dingtalk': return 'orange'
    case 'qq': return 'green'
    default: return 'primary'
  }
}

function botTypeLabel (type?: BotType): string {
  switch (type) {
    case 'lark': return '飞书'
    case 'dingtalk': return '钉钉'
    case 'qq': return 'QQ'
    default: return 'Telegram'
  }
}

function botApiPrefix (type: BotType): string {
  switch (type) {
    case 'lark': return '/larkbots'
    case 'telegram': return '/telegrambots'
    case 'dingtalk': return '/dingtalkbots'
    case 'qq': return '/qqbots'
  }
}

export function useBotPage () {
  const { t } = useI18n()
  const $q = useQuasar()

  const loading = ref(false)
  const rowBusyAgentId = ref<number | null>(null)
  const bots = ref<BotRow[]>([])
  const botCount = ref(0)
  const running = ref(false)

  const historyDialogOpen = ref(false)
  const historyAgentId = ref(0)
  const historyAgentName = ref('')
  const historyChannel = ref<'lark' | 'telegram' | 'dingtalk' | 'qq' | 'wecom' | 'all'>('all')

  const columns = computed(() => {
    const cols = [
      { name: 'agent_name', label: t('agentName'), field: 'agent_name', align: 'left' as const },
      { name: 'agent_id', label: 'Agent ID', field: 'agent_id', align: 'center' as const },
      { name: 'bot_type', label: t('type'), field: 'bot_type', align: 'center' as const },
      { name: 'app_id', label: 'App ID', field: 'app_id', align: 'center' as const },
      { name: 'is_running', label: t('status'), field: 'is_running', align: 'center' as const },
      { name: 'ops', label: t('actions'), field: 'agent_id', align: 'center' as const }
    ]
    return cols
  })

  async function load () {
    loading.value = true
    try {
      const [larkRes, telegramRes, dingtalkRes, qqRes] = await Promise.all([
        api.get<APIResponse<BotListResponse>>('/larkbots'),
        api.get<APIResponse<BotListResponse>>('/telegrambots'),
        api.get<APIResponse<BotListResponse>>('/dingtalkbots'),
        api.get<APIResponse<BotListResponse>>('/qqbots')
      ])
      const larkBots = ((larkRes.data.data as BotListResponse)?.bots || []).map(b => ({ ...b, bot_type: 'lark' as BotType }))
      const telegramBots = ((telegramRes.data.data as BotListResponse)?.bots || []).map(b => ({ ...b, bot_type: 'telegram' as BotType }))
      const dingtalkBots = ((dingtalkRes.data.data as BotListResponse)?.bots || []).map(b => ({ ...b, bot_type: 'dingtalk' as BotType }))
      const qqBots = ((qqRes.data.data as BotListResponse)?.bots || []).map(b => ({ ...b, bot_type: 'qq' as BotType }))
      const allBots = [...larkBots, ...telegramBots, ...dingtalkBots, ...qqBots]
      bots.value = allBots
      botCount.value = allBots.length
      running.value =
        ((larkRes.data.data as BotListResponse)?.running ||
        (telegramRes.data.data as BotListResponse)?.running ||
        (dingtalkRes.data.data as BotListResponse)?.running ||
        (qqRes.data.data as BotListResponse)?.running) || false
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('loadFailed') })
    } finally {
      loading.value = false
    }
  }

  async function startAll () {
    loading.value = true
    try {
      await Promise.all([
        api.post('/larkbots/start'),
        api.post('/telegrambots/start'),
        api.post('/dingtalkbots/start'),
        api.post('/qqbots/start')
      ])
      $q.notify({ type: 'positive', message: t('botStarted') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      loading.value = false
    }
  }

  async function stopAll () {
    loading.value = true
    try {
      await Promise.all([
        api.post('/larkbots/stop'),
        api.post('/telegrambots/stop'),
        api.post('/dingtalkbots/stop'),
        api.post('/qqbots/stop')
      ])
      $q.notify({ type: 'positive', message: t('botStopped') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      loading.value = false
    }
  }

  async function unregister (agentId: number, type: BotType) {
    rowBusyAgentId.value = agentId
    try {
      await api.delete(`${botApiPrefix(type)}/${agentId}`)
      $q.notify({ type: 'positive', message: t('botUnregistered') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      rowBusyAgentId.value = null
    }
  }

  async function startRowAgent (agentId: number, type: BotType) {
    rowBusyAgentId.value = agentId
    try {
      await api.post(`${botApiPrefix(type)}/${agentId}/ws/start`)
      $q.notify({ type: 'positive', message: t('botRowStarted') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      rowBusyAgentId.value = null
    }
  }

  async function stopRowAgent (agentId: number, type: BotType) {
    rowBusyAgentId.value = agentId
    try {
      await api.post(`${botApiPrefix(type)}/${agentId}/ws/stop`)
      $q.notify({ type: 'positive', message: t('botRowStopped') })
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      rowBusyAgentId.value = null
    }
  }

  function openHistory (row: BotRow) {
    historyAgentId.value = row.agent_id
    historyAgentName.value = row.agent_name || ''
    historyChannel.value = row.bot_type ?? 'all'
    historyDialogOpen.value = true
  }

  function confirmUnregister (agentId: number, type: BotType, agentName: string) {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${t('botUnregisterConfirm')}: ${agentName} (Agent ID: ${agentId})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('confirm'), color: 'negative' }
    }).onOk(async () => {
      await unregister(agentId, type)
    })
  }

  onMounted(() => { void load() })

  return {
    t,
    loading,
    rowBusyAgentId,
    bots,
    botCount,
    running,
    columns,
    load,
    startAll,
    stopAll,
    unregister,
    confirmUnregister,
    startRowAgent,
    stopRowAgent,
    historyDialogOpen,
    historyAgentId,
    historyAgentName,
    historyChannel,
    openHistory,
    botTypeColor,
    botTypeLabel
  }
}
