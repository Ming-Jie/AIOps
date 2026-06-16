<template>
  <q-page class="team-chat-page column no-wrap" :style-fn="teamChatPageStyleFn">
    <q-banner v-if="errorMsg" class="bg-negative text-white" dense>{{ errorMsg }}</q-banner>

    <!-- Team info header -->
    <div v-if="team" class="row items-center team-header">
      <q-btn flat round dense icon="arrow_back" @click="goBack" class="q-mr-sm" />
      <q-icon :name="modeIcon(team.mode)" size="sm" color="primary" class="q-mr-sm" />
      <div class="text-h6 text-text2">{{ team.name }}</div>
      <q-chip dense class="q-ml-sm" size="sm" color="primary" text-color="white">{{ modeLabel(team.mode) }}</q-chip>
      <q-space />
      <q-btn flat dense :label="t('teamMembers')" icon="groups" @click="showMembers = !showMembers" />
    </div>

    <!-- Members panel (collapsible) -->
    <q-slide-transition>
      <div v-if="showMembers && team">
        <q-card flat bordered class="q-mb-sm">
          <q-card-section class="q-py-sm">
            <div class="text-caption text-grey">{{ t('teamMembers') }}</div>
            <div class="row q-mt-xs q-gutter-xs">
              <q-chip
                v-for="m in team.members" :key="m.id" dense color="secondary" text-color="white"
                :icon="m.agent_id === team.coordinator_agent_id ? 'star' : 'smart_toy'"
              >
                {{ m.agent_name || `Agent(${m.agent_id})` }}
                <q-tooltip v-if="m.agent_id === team.coordinator_agent_id">{{ t('teamCoordinator') }}</q-tooltip>
              </q-chip>
            </div>
          </q-card-section>
        </q-card>
      </div>
    </q-slide-transition>

    <!-- Conversations sidebar + chat area -->
    <div class="row team-main col no-wrap min-height-0">
      <!-- Conversations list -->
      <aside class="team-convs column no-wrap chat-session-rail">
        <div class="row items-center no-wrap q-px-xs q-pt-sm q-pb-none team-convs-toolbar">
          <div class="text-caption text-text3 col ellipsis">{{ t('teamConversations') }}</div>
          <q-btn
            round dense flat color="primary" icon="add"
            :aria-label="t('teamStartConversation')"
            @click="createConversation"
          />
        </div>
        <q-separator />
        <q-scroll-area class="chat-session-rail-scroll col">
          <q-list padding dense class="chat-session-grouped-list">
            <div v-if="conversations.length === 0" class="text-caption text-text3 q-pa-md text-center">
              {{ t('teamNoMessages') }}
            </div>
            <q-item
              v-for="conv in conversations" :key="conv.id"
              :active="currentConv?.id === conv.id"
              active-class="chat-session-item-active"
              class="chat-session-row"
              tabindex="-1"
            >
              <q-item-section clickable v-ripple class="col min-width-0" @click="selectConversation(conv)">
                <q-item-label class="ellipsis">{{ conv.title }}</q-item-label>
                <q-item-label caption class="ellipsis">{{ formatTime(conv.created_at) }}</q-item-label>
              </q-item-section>
              <q-item-section v-if="conv.round > 0" side>
                <q-badge color="primary" rounded>{{ conv.round }}</q-badge>
              </q-item-section>
            </q-item>
          </q-list>
        </q-scroll-area>
      </aside>

      <!-- Chat area -->
      <div class="team-chat-area column no-wrap col min-width-0 min-height-0">
        <div v-if="currentConv" class="team-chat-panel column no-wrap col min-height-0">
          <div ref="msgContainer" class="team-msg-list col min-height-0">
            <div class="team-msg-inner">
              <div v-if="messages.length === 0" class="text-center text-grey q-mt-lg">
                {{ t('teamNoMessages') }}
              </div>

              <div v-for="msg in messages" :key="msg.id" class="q-mb-sm">
                <!-- User message -->
                <div v-if="msg.msg_type === 'user_input'" class="row justify-end q-mb-xs">
                  <div class="chat-bubble chat-bubble-user">
                    <div class="text-caption text-weight-medium q-mb-xs">{{ t('teamChatUser') }}</div>
                    <div class="text-body2">{{ msg.content }}</div>
                  </div>
                </div>

                <!-- System/coordination message -->
                <div v-else-if="msg.msg_type === 'coordination'" class="row justify-center q-mb-xs">
                  <div class="chat-bubble chat-bubble-system">
                    <div class="text-caption text-weight-medium text-primary">{{ t('teamChatCoordination') }}</div>
                    <div class="text-body2 team-md-body">
                      <TeamMdRenderer :content="msg.content" />
                    </div>
                  </div>
                </div>

                <!-- Summary message -->
                <div v-else-if="msg.msg_type === 'summary'" class="row justify-center q-mb-xs">
                  <div class="chat-bubble chat-bubble-summary">
                    <div class="text-caption text-weight-medium text-positive">{{ t('teamChatSummary') }}</div>
                    <div class="text-body2 team-md-body">
                      <TeamMdRenderer :content="msg.content" />
                    </div>
                  </div>
                </div>

                <!-- Debate message -->
                <div v-else-if="msg.msg_type === 'debate'" class="q-mb-xs">
                  <div class="chat-bubble chat-bubble-agent">
                    <div class="row items-center q-mb-xs">
                      <q-icon name="forum" size="xs" color="purple" class="q-mr-xs" />
                      <div class="text-caption text-weight-medium text-purple">{{ msg.sender_name }}</div>
                    </div>
                    <div class="text-body2 team-md-body">
                      <TeamMdRenderer :content="msg.content" />
                    </div>
                  </div>
                </div>

                <!-- Routing message -->
                <div v-else-if="msg.msg_type === 'routing'" class="row justify-center q-mb-xs">
                  <div class="chat-bubble chat-bubble-system">
                    <div class="text-caption text-weight-medium text-secondary">{{ t('teamChatRouting') }}</div>
                    <div class="text-body2 team-md-body">
                      <TeamMdRenderer :content="msg.content" />
                    </div>
                  </div>
                </div>

                <!-- Agent response -->
                <div v-else class="q-mb-xs">
                  <div class="chat-bubble chat-bubble-agent">
                    <div class="row items-center q-mb-xs">
                      <q-icon name="smart_toy" size="xs" color="primary" class="q-mr-xs" />
                      <div class="text-caption text-weight-medium text-primary">{{ msg.sender_name }}</div>
                      <template v-if="msg.metadata?.step">
                        <q-chip dense size="xs" class="q-ml-xs">Step {{ msg.metadata.step }}/{{ msg.metadata.total_steps }}</q-chip>
                      </template>
                    </div>
                    <div class="text-body2 team-md-body">
                      <TeamMdRenderer :content="msg.content" />
                    </div>
                  </div>
                </div>
              </div>

              <!-- Loading indicator -->
              <div v-if="sending" class="row justify-center q-my-md">
                <q-spinner-dots color="primary" size="2rem" />
                <div class="text-caption text-grey q-ml-sm">{{ t('teamChatTeamThinking') }}</div>
              </div>
            </div>
          </div>

          <!-- Input -->
          <div class="q-mt-sm row items-center team-input">
            <q-input
              v-model="inputText" :placeholder="t('teamChatPlaceholder')" outlined dense
              class="col" autogrow @keyup.enter="sendMessage"
              :disable="sending"
            />
            <q-btn
              color="primary" icon="send" round dense class="q-ml-sm"
              @click="sendMessage" :loading="sending" :disable="!inputText.trim()"
            />
          </div>
        </div>

        <div v-else class="col column flex-center text-grey team-empty-state">
          <q-icon name="chat" size="48px" />
          <div class="q-mt-sm text-body2">{{ t('teamNoMessages') }}</div>
        </div>
      </div>
    </div>
  </q-page>
</template>

<script setup lang="ts">
import TeamMdRenderer from 'components/TeamMdRenderer.vue'
import { useTeamChatPage } from './useTeamChatPage'

defineOptions({ name: 'TeamChatPage' })

function teamChatPageStyleFn (offset: number, height: number): Record<string, string> {
  const h = height - offset
  return {
    minHeight: `${h}px`,
    maxHeight: `${h}px`,
    height: `${h}px`,
    overflow: 'hidden'
  }
}

const {
  t, team, errorMsg, sending, showMembers,
  conversations, currentConv, messages, inputText,
  goBack, createConversation, selectConversation,
  sendMessage, modeLabel, modeIcon, formatTime,
  msgContainer
} = useTeamChatPage()
</script>

<style scoped>
.team-chat-page {
  overflow: hidden;
  box-sizing: border-box;
}
.team-header {
  flex-shrink: 0;
  padding: 8px 16px;
}
.team-main {
  flex: 1 1 0;
  min-height: 0;
  min-width: 0;
  padding: 0 16px 16px;
  overflow: hidden;
}
.team-convs {
  min-height: 0;
  overflow: hidden;
}
.team-convs-toolbar {
  flex-shrink: 0;
}
.team-chat-area {
  flex: 1 1 0;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
.team-chat-panel {
  min-height: 0;
  overflow: hidden;
}
.team-msg-list {
  flex: 1 1 0;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
  padding: 4px 8px;
  background: #f5f5f7;
}
.team-msg-inner {
  width: 100%;
  max-width: 720px;
  margin: 0 auto;
  box-sizing: border-box;
}
.team-input {
  flex-shrink: 0;
  max-width: 720px;
  width: 100%;
  margin: 0 auto;
  padding: 0 8px 4px;
  box-sizing: border-box;
}
.team-empty-state {
  min-height: 0;
}

.chat-bubble {
  max-width: min(88%, 560px);
  padding: 8px 12px;
  border-radius: 12px;
  word-break: break-word;
}
.chat-bubble-user {
  max-width: min(94%, 480px);
  background: #1976d2;
  color: white;
  border-bottom-right-radius: 4px;
}
.chat-bubble-agent {
  background: #f0f0f0;
  border-bottom-left-radius: 4px;
}
.chat-bubble-system {
  background: #e3f2fd;
  border-radius: 12px;
  max-width: min(92%, 640px);
}
.chat-bubble-summary {
  background: #e8f5e9;
  border-radius: 12px;
  max-width: min(92%, 640px);
}
.team-md-body {
  line-height: 1.5;
}
.team-md-body :deep(.chat-md-root h1),
.team-md-body :deep(.chat-md-root h2),
.team-md-body :deep(.chat-md-root h3),
.team-md-body :deep(.chat-md-root h4) {
  font-size: 0.95rem;
  font-weight: 600;
  margin: 0.5em 0 0.25em;
  line-height: 1.35;
}
.team-md-body :deep(.chat-md-root h1:first-child),
.team-md-body :deep(.chat-md-root h2:first-child),
.team-md-body :deep(.chat-md-root h3:first-child),
.team-md-body :deep(.chat-md-root h4:first-child) {
  margin-top: 0;
}
.team-md-body :deep(.chat-md-root p),
.team-md-body :deep(.chat-md-root li) {
  font-size: inherit;
}
.team-md-body :deep(.chat-md-root img) {
  max-width: 100%;
  border-radius: 6px;
  margin: 4px 0;
  display: block;
}
</style>
