<template>
  <q-dialog :model-value="open" persistent maximized @update:model-value="onDialogUpdate">
    <q-card class="column full-height">
      <q-card-section class="row items-center q-py-sm">
        <div class="text-h6">
          {{ t('botHistoryTitle') }}
          <span v-if="agentName" class="text-weight-regular text-grey-8"> — {{ agentName }}</span>
        </div>
        <q-space />
        <q-btn flat round dense icon="close" :aria-label="t('cancel')" @click="close" />
      </q-card-section>
      <q-separator />
      <q-card-section class="col row no-wrap q-pa-none min-height-0">
        <div class="bot-history-sessions column no-wrap">
          <div class="q-pa-sm row items-center no-wrap">
            <q-select
              v-model="channelFilter"
              :options="channelOptions"
              dense
              outlined
              emit-value
              map-options
              class="col"
              :label="t('botHistoryChannel')"
              @update:model-value="reloadSessions"
            />
            <q-btn flat round dense icon="refresh" class="q-ml-xs" :loading="sessionsLoading" @click="reloadSessions" />
          </div>
          <q-separator />
          <q-scroll-area class="col">
            <q-list dense padding>
              <q-inner-loading :showing="sessionsLoading" />
              <div v-if="!sessionsLoading && sessions.length === 0" class="text-caption text-grey-7 q-pa-md">
                {{ t('botHistoryNoSessions') }}
              </div>
              <q-item
                v-for="s in sessions"
                :key="s.session_id"
                :active="selectedSessionId === s.session_id"
                active-class="bg-blue-1"
                clickable
                v-ripple
                @click="selectSession(s.session_id)"
              >
                <q-item-section>
                  <q-item-label class="ellipsis">{{ sessionLabel(s) }}</q-item-label>
                  <q-item-label caption class="ellipsis">{{ s.im_user_id }}</q-item-label>
                  <q-item-label caption>{{ formatTime(s.updated_at) }}</q-item-label>
                </q-item-section>
              </q-item>
            </q-list>
          </q-scroll-area>
        </div>
        <q-separator vertical />
        <div class="col column no-wrap min-width-0">
          <div v-if="!selectedSessionId" class="col flex flex-center text-grey-6">
            {{ t('botHistorySelectSession') }}
          </div>
          <template v-else>
            <div class="row items-center q-px-md q-pt-sm q-pb-none">
              <q-btn
                flat
                dense
                color="primary"
                icon="open_in_new"
                :label="t('botHistoryOpenInChat')"
                @click="openInChat"
              />
            </div>
            <q-scroll-area class="col q-pa-md">
              <q-inner-loading :showing="messagesLoading" />
              <div v-if="!messagesLoading && messages.length === 0" class="text-caption text-grey-7">
                {{ t('botHistoryNoMessages') }}
              </div>
              <div v-for="(m, idx) in messages" :key="idx" class="q-mb-md">
                <div class="text-caption text-grey-7 q-mb-xs">
                  {{ m.role === 'user' ? t('chatRoleUser') : t('chatRoleAgent') }}
                  <span v-if="m.created_at" class="q-ml-sm">{{ formatTime(m.created_at) }}</span>
                </div>
                <div :class="['bot-history-bubble', m.role === 'user' ? 'bot-history-bubble-user' : 'bot-history-bubble-assistant']">
                  <MdRenderer v-if="m.role === 'assistant'" :content="m.content" />
                  <div v-else class="text-body2" style="white-space: pre-wrap">{{ m.content }}</div>
                </div>
              </div>
            </q-scroll-area>
          </template>
        </div>
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { api } from 'boot/axios'
import { ROUTE_SESSION_Q } from 'src/pages/chat/chatRouteQuery'
import MdRenderer from 'components/MdRenderer.vue'
import type { APIResponse, ChatHistoryMessage } from 'src/api/types'

export interface IMChatSession {
  session_id: string
  agent_id: number
  user_id?: string
  title?: string
  im_channel: string
  im_user_id: string
  created_at: string
  updated_at: string
}

const props = defineProps<{
  open: boolean
  agentId: number
  agentName?: string
  defaultChannel?: 'lark' | 'telegram' | 'dingtalk' | 'all'
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { t } = useI18n()
const router = useRouter()

function openInChat () {
  const sid = selectedSessionId.value
  if (!sid || props.agentId < 1) return
  void router.push({
    name: 'chat',
    params: { agentId: String(props.agentId) },
    query: { [ROUTE_SESSION_Q]: sid }
  })
  close()
}

const channelFilter = ref<'lark' | 'telegram' | 'dingtalk' | 'all'>('all')
const channelOptions = computed(() => [
  { label: t('botHistoryChannelAll'), value: 'all' as const },
  { label: t('imEnabledOptionsLark'), value: 'lark' as const },
  { label: t('imEnabledOptionsTelegram'), value: 'telegram' as const },
  { label: t('imEnabledOptionsDingtalk'), value: 'dingtalk' as const }
])

const sessions = ref<IMChatSession[]>([])
const sessionsLoading = ref(false)
const selectedSessionId = ref<string | null>(null)
const messages = ref<ChatHistoryMessage[]>([])
const messagesLoading = ref(false)

function close () {
  emit('update:open', false)
}

function onDialogUpdate (v: boolean) {
  if (!v) close()
}

function formatTime (iso: string): string {
  if (!iso) return ''
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}

function sessionLabel (s: IMChatSession): string {
  const title = (s.title || '').trim()
  if (title) return title
  const ch =
    s.im_channel === 'lark'
      ? t('imEnabledOptionsLark')
      : s.im_channel === 'telegram'
        ? t('imEnabledOptionsTelegram')
        : s.im_channel === 'dingtalk'
          ? t('imEnabledOptionsDingtalk')
          : s.im_channel
  return `${ch} · ${s.im_user_id}`
}

async function reloadSessions () {
  if (props.agentId < 1) return
  sessionsLoading.value = true
  selectedSessionId.value = null
  messages.value = []
  try {
    const { data } = await api.get<APIResponse<IMChatSession[]>>('/imbots/sessions', {
      params: {
        agent_id: props.agentId,
        channel: channelFilter.value,
        limit: 100
      }
    })
    sessions.value = (data.data ?? []) as IMChatSession[]
  } catch {
    sessions.value = []
  } finally {
    sessionsLoading.value = false
  }
}

async function selectSession (sessionId: string) {
  if (selectedSessionId.value === sessionId) return
  selectedSessionId.value = sessionId
  messagesLoading.value = true
  try {
    const { data } = await api.get<APIResponse<ChatHistoryMessage[]>>(
      `/imbots/sessions/${encodeURIComponent(sessionId)}/messages`,
      { params: { agent_id: props.agentId, limit: 200 } }
    )
    messages.value = (data.data ?? []) as ChatHistoryMessage[]
  } catch {
    messages.value = []
  } finally {
    messagesLoading.value = false
  }
}

watch(
  () => [props.open, props.agentId, props.defaultChannel] as const,
  ([isOpen, agentId, defCh]) => {
    if (!isOpen || agentId < 1) return
    channelFilter.value = defCh ?? 'all'
    void reloadSessions()
  },
  { immediate: true }
)
</script>

<style scoped>
.bot-history-sessions {
  width: min(320px, 38vw);
  min-width: 240px;
  border-right: 1px solid rgba(0, 0, 0, 0.08);
}
.bot-history-bubble {
  border-radius: 8px;
  padding: 10px 12px;
  max-width: 100%;
  overflow-wrap: anywhere;
}
.bot-history-bubble-user {
  background: #e3f2fd;
}
.bot-history-bubble-assistant {
  background: #f5f5f5;
}
.min-height-0 {
  min-height: 0;
}
</style>
