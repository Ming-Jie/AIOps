<template>
  <q-page class="chat-page chat-page-layout column no-wrap" :style-fn="chatPageStyleFn">
    <div class="chat-toolbar row items-center q-px-md q-py-sm no-wrap">
      <q-btn
        v-if="showSessionRail && sessionRailCollapsed"
        flat
        round
        dense
        color="primary"
        icon="chevron_right"
        class="q-mr-xs"
        :aria-label="t('chatSessionRailExpand')"
        @click="toggleSessionRailCollapse"
      />
      <q-btn
        v-if="!showSessionRail"
        flat
        round
        dense
        color="primary"
        icon="history"
        class="q-mr-xs"
        :aria-label="t('chatOpenSessions')"
        @click="sessionDrawerOpen = true"
      />
      <q-select
        v-model="agentId"
        class="chat-toolbar-select"
        :options="agentOptions"
        option-value="id"
        option-label="label"
        emit-value
        map-options
        outlined
        dense
        :label="t('selectAgent')"
        :loading="agentsLoading"
      />
      <q-space />
      <q-btn
        v-if="sending"
        flat
        dense
        no-caps
        outline
        color="negative"
        icon="stop"
        :label="$q.screen.gt.sm ? t('chatStopReply') : undefined"
        :loading="stopping"
        :disable="stopping"
        class="q-mr-xs"
        @click="stopStream"
      />
      <q-btn
        flat
        round
        dense
        :color="thoughtSidebarOpen ? 'primary' : 'grey-7'"
        icon="psychology"
        :title="thoughtSidebarOpen ? '隐藏思考过程' : '显示思考过程'"
        @click="toggleThoughtSidebar"
      />
      <q-btn outline color="primary" :label="t('newSession')" :disable="!canStartSession" :loading="sessionBusy" @click="createSessionFromSidebar" />
    </div>

    <div class="chat-main-body row col no-wrap min-height-0">
      <aside v-if="showSessionRail && !sessionRailCollapsed" class="chat-session-rail column no-wrap">
        <div class="row items-center no-wrap q-px-xs q-pt-sm q-pb-none">
          <q-btn
            round
            dense
            flat
            color="grey-7"
            icon="chevron_left"
            :aria-label="t('chatSessionRailCollapse')"
            @click="toggleSessionRailCollapse"
          />
          <q-btn
            round
            dense
            flat
            color="primary"
            icon="add"
            :disable="!canStartSession"
            :loading="sessionBusy"
            :aria-label="t('newSession')"
            @click="createSessionFromSidebar"
          />
        </div>
        <q-separator />
        <q-scroll-area class="chat-session-rail-scroll col">
          <q-list padding dense class="chat-session-grouped-list">
            <q-inner-loading :showing="sessionsLoading" />
            <div v-if="!sessionsLoading && sessionsList.length === 0" class="text-caption text-text3 q-pa-md">
              {{ t('chatNoSessions') }}
            </div>
            <template v-for="(block, bIdx) in sessionRailPreviewBlocks" :key="block.key">
              <q-item-label
                header
                :class="['chat-session-group-label', 'text-caption', 'text-text3', bIdx === 0 ? 'chat-session-group-label--first' : '']"
              >
                {{ block.label }}
              </q-item-label>
              <q-item
                v-for="s in block.items"
                :key="'rail-' + block.key + '-' + s.session_id"
                :active="sessionId === s.session_id"
                active-class="chat-session-item-active"
                class="chat-session-row"
                tabindex="-1"
              >
                <q-item-section clickable v-ripple class="col min-width-0" @click="selectSession(s.session_id)">
                  <q-item-label class="ellipsis">{{ sessionTitle(s) }}</q-item-label>
                  <q-item-label caption class="ellipsis">{{ formatSessionTime(s.updated_at) }}</q-item-label>
                </q-item-section>
                <q-item-section side class="chat-session-row-actions">
                  <div class="row items-center no-wrap">
                    <q-btn
                      flat
                      round
                      dense
                      size="sm"
                      icon="edit"
                      color="grey-7"
                      class="chat-session-row-edit"
                      :aria-label="t('rename')"
                      @click.stop="promptRenameSession(s)"
                    />
                    <q-btn
                      flat
                      round
                      dense
                      size="sm"
                      icon="delete"
                      color="negative"
                      class="chat-session-row-delete"
                      :aria-label="t('delete')"
                      @click.stop="confirmDeleteSession(s)"
                    />
                  </div>
                </q-item-section>
              </q-item>
            </template>
          </q-list>
        </q-scroll-area>
        <div v-if="showViewAllSessions" class="chat-session-rail-footer q-px-xs q-pb-xs row justify-center">
          <q-btn
            flat
            dense
            no-caps
            color="primary"
            class="text-caption"
            :label="t('chatViewAllSessions')"
            @click="sessionFullDialogOpen = true"
          />
        </div>
      </aside>

      <div class="chat-main-right row col min-width-0 min-height-0 no-wrap relative-position">
        <div class="chat-main-center column col min-width-0 min-height-0 no-wrap">
          <div ref="chatScrollRef" class="chat-scroll-area col">
            <div
              v-if="!sessionId && displayMessages.length === 0"
              class="chat-welcome column items-center justify-center text-center"
            >
              <div class="chat-welcome-icon flex flex-center">
                <q-icon name="chat" size="42px" color="primary" />
              </div>
              <div class="text-h5 text-weight-bold q-mt-md">{{ t('chatWelcomeTitle') }}</div>
              <p class="chat-welcome-sub text-text3 q-mt-sm q-mb-lg">{{ t('chatWelcomeSubtitle') }}</p>
              <div class="chat-quick-grid row q-col-gutter-md">
                <div class="col-12 col-sm-6">
                  <q-card flat bordered class="chat-quick-card cursor-pointer" @click="fillDraft(t('chatQuickPrompt1'))">
                    <q-card-section class="text-left">
                      <q-icon name="summarize" color="primary" class="q-mb-sm" size="sm" />
                      <div class="text-body2 text-weight-medium">{{ t('chatQuickPrompt1') }}</div>
                    </q-card-section>
                  </q-card>
                </div>
                <div class="col-12 col-sm-6">
                  <q-card flat bordered class="chat-quick-card cursor-pointer" @click="fillDraft(t('chatQuickPrompt2'))">
                    <q-card-section class="text-left">
                      <q-icon name="bug_report" color="orange-8" class="q-mb-sm" size="sm" />
                      <div class="text-body2 text-weight-medium">{{ t('chatQuickPrompt2') }}</div>
                    </q-card-section>
                  </q-card>
                </div>
              </div>
            </div>

            <div v-else class="chat-messages q-px-md q-py-md">
              <div v-if="displayMessages.length === 0" class="text-center text-text3 q-py-xl">
                {{ t('chatEmptyThread') }}
              </div>
              <template v-for="(m, idx) in displayMessages" :key="idx">
                <div v-if="!shouldHideAssistantPlanMessage(idx)" class="chat-message-turn">
                  <div
                    v-if="messageDateDividerAt(displayMessages, idx)"
                    class="row justify-center q-mb-sm q-mt-xs"
                  >
                    <span class="text-caption text-text3 chat-date-divider">{{ messageDateDividerAt(displayMessages, idx) }}</span>
                  </div>
                  <div
                    class="chat-message-row row no-wrap q-mb-md"
                    :class="m.role === 'user' ? 'justify-end' : 'justify-start'"
                  >
                    <div
                      class="column chat-bubble-column"
                      :class="[
                        m.role === 'user' ? 'items-end chat-bubble-column-user' : 'items-start chat-bubble-column-assistant'
                      ]"
                    >
                      <div v-if="chatMessageTimeLabel(m)" class="text-caption text-text3 q-mb-xs chat-msg-time">
                        {{ chatMessageTimeLabel(m) }}
                      </div>
                      <div :class="['chat-bubble', m.role === 'user' ? 'chat-bubble-user' : 'chat-bubble-assistant']">
                        <div class="text-caption text-text3 q-mb-xs">
                          <template v-if="m.role === 'user'">
                            {{ t('chatRoleUser') }}
                          </template>
                          <template v-else-if="(m as any).agentId && agents">
                            {{ agents.find((a: any) => a.id === (m as any).agentId)?.name || t('chatRoleAgent') }}
                          </template>
                          <template v-else>{{ t('chatRoleAgent') }}</template>
                        </div>
                        <div
                          v-if="m.role === 'user' && userMessageImageUrls(m).length > 0"
                          class="chat-bubble-user-images row wrap q-gutter-xs q-mb-sm"
                        >
                          <div
                            v-for="(u, imgIdx) in userMessageImageUrls(m)"
                            :key="imgIdx"
                            class="chat-bubble-img-wrap"
                          >
                            <img
                              :src="resolveChatImageUrl(u)"
                              class="chat-bubble-img-display chat-bubble-img--zoom"
                              alt=""
                              loading="eager"
                              :title="t('chatImageClickHint')"
                              @click="openImagePreview(u)"
                            >
                          </div>
                        </div>
                        <div v-if="m.role === 'user' && m.file_urls && m.file_urls.length > 0" class="chat-bubble-files q-mb-sm">
                          <div
                            v-for="(url, fIdx) in m.file_urls"
                            :key="fIdx"
                            class="chat-bubble-file row items-center q-mb-xs cursor-pointer"
                            @click="openFilePreview(url)"
                          >
                            <q-icon :name="getFileIcon(url)" size="sm" color="primary" class="q-mr-xs" />
                            <span class="text-body2 ellipsis">{{ getFileName(url) }}</span>
                          </div>
                        </div>
                        <div class="chat-bubble-body text-body2">
                          <template v-if="m.role === 'user'">{{ userMessageTextToDisplay(m) }}</template>
                          <template v-else>
                            <div
                              v-if="isAssistantTypingSlot(idx)"
                              class="chat-typing"
                              role="status"
                              aria-live="polite"
                            >
                              <span class="chat-typing-dot" />
                              <span class="chat-typing-dot" />
                              <span class="chat-typing-dot" />
                            </div>
                            <PlanExecutePanel
                              v-else-if="shouldRenderPlanExecuteForMessage(idx)"
                              :tasks="getPlanExecuteTasksFromMessage(m as any)"
                              :title="t('chatPlanExecuteTaskList')"
                            />
                            <PlanExecutePanel
                              v-else-if="sending && idx === displayMessages.length - 1 && getCurrentPlanTasks().length > 0"
                              :tasks="getCurrentPlanTasks()"
                              :title="t('chatPlanExecuteTaskList')"
                            />
                            <MdRenderer
                              v-if="(m.displayedContent || m.content || '').trim() && !shouldHideAssistantMessageText(idx)"
                              :content="m.displayedContent || m.content"
                            />
                          </template>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </template>
            </div>
          </div>

          <div
            v-if="pendingImages.length > 0 || pendingFiles.length > 0"
            class="chat-pending-strip q-px-md q-pt-sm q-pb-md"
          >
            <div v-if="pendingImages.length > 0" class="chat-pending-image q-mb-md">
              <div class="chat-pending-image-card">
                <div class="row items-center justify-between no-wrap q-mb-sm">
                  <span class="text-caption text-weight-medium text-text2">{{ t('chatPendingImageCaption') }}</span>
                  <q-btn
                    flat
                    dense
                    no-caps
                    size="sm"
                    color="negative"
                    icon="delete_outline"
                    :label="t('chatClearAllImages')"
                    @click="clearPendingImages"
                  />
                </div>
                <div class="chat-pending-thumbs">
                  <div
                    v-for="(pi, pIdx) in pendingImages"
                    :key="pIdx"
                    class="chat-pending-thumb-cell"
                  >
                    <div class="chat-pending-thumb-wrap">
                      <img
                        :src="pi.dataUrl"
                        class="chat-pending-thumb chat-pending-thumb--zoom"
                        alt=""
                        :title="t('chatImageClickHint')"
                        @click="openImagePreview(pi.dataUrl)"
                      >
                      <q-btn
                        class="chat-pending-thumb-remove"
                        flat
                        round
                        dense
                        size="sm"
                        icon="close"
                        color="white"
                        :aria-label="t('chatClearImage')"
                        @click.stop="removePendingImageAt(pIdx)"
                      />
                    </div>
                  </div>
                </div>
                <div class="text-caption text-text3 q-mt-sm">{{ t('chatImageHint') }}</div>
              </div>
            </div>
            <div v-if="pendingFiles.length > 0" class="chat-pending-files">
              <div class="chat-pending-files-card">
                <div class="text-caption text-weight-medium text-text2 q-mb-sm">{{ t('chatPendingFilesCaption') }}</div>
                <div
                  v-for="(f, idx) in pendingFiles"
                  :key="idx"
                  class="chat-pending-file row items-center no-wrap q-mb-xs"
                >
                  <q-icon name="attach_file" size="sm" color="grey-7" class="q-mr-xs chat-pending-file-icon" />
                  <div class="col text-caption text-text3 ellipsis">
                    {{ f.name }}
                    <span v-if="f.size > 0" class="text-text2"> · {{ formatPendingFileSize(f.size) }}</span>
                  </div>
                  <q-btn
                    flat
                    dense
                    no-caps
                    size="sm"
                    color="negative"
                    icon="delete_outline"
                    :label="t('chatClearFile')"
                    @click="removePendingFile(idx)"
                  />
                </div>
                <q-btn
                  v-if="pendingFiles.length > 1"
                  flat
                  dense
                  no-caps
                  size="sm"
                  color="primary"
                  class="q-mt-xs"
                  :label="t('chatClearAllFiles')"
                  @click="clearPendingFiles"
                />
              </div>
            </div>
          </div>

          <div class="chat-composer q-px-md q-py-md">
            <div
              class="chat-composer-inner row items-end no-wrap"
              @paste="onComposerPaste"
              @dragover="onComposerDragOver"
              @drop="onComposerDrop"
            >
              <div class="col column relative-position min-width-0 self-center">
                <div class="chat-composer-field-wrap col row items-stretch no-wrap">
                  <div class="chat-composer-attach row items-center no-wrap flex-shrink-0">
                    <div class="row items-center no-wrap chat-composer-prepend">
                      <div class="chat-attach-slot relative-position">
                        <q-icon name="attach_file" size="sm" color="grey-7" class="chat-attach-under" />
                        <input
                          ref="imageInputRef"
                          type="file"
                          class="chat-attach-native"
                          :accept="chatImageInputAccept"
                          multiple
                          :disabled="!canStartSession"
                          :aria-label="t('chatAttachImageAria')"
                          @change="onImageSelected"
                        >
                      </div>
                      <div class="chat-attach-slot relative-position">
                        <q-icon name="folder_open" size="sm" color="grey-7" class="chat-attach-under" />
                        <input
                          ref="fileInputRef"
                          type="file"
                          class="chat-attach-native"
                          :accept="chatDocumentInputAccept"
                          multiple
                          :disabled="!canStartSession"
                          :aria-label="t('chatAttachFileAria')"
                          @change="onFileSelected"
                        >
                      </div>
                    </div>
                  </div>
                  <div class="col column min-width-0 self-center">
                    <q-input
                      ref="composerInputRef"
                      v-model="draft"
                      class="col chat-input chat-input-with-attach"
                      type="textarea"
                      autogrow
                      borderless
                      dense
                      :placeholder="t('messagePlaceholder')"
                      :disable="!canStartSession"
                      @keydown="onComposerKeydown"
                    />
                  </div>
                </div>
              </div>
              <q-btn
                class="flex-shrink-0"
                :color="sending ? 'negative' : 'primary'"
                unelevated
                padding="sm md"
                :label="sending ? t('chatStopReply') : t('send')"
                :disable="(!canSend && !sending) || stopping"
                :loading="stopping"
                @click="sending ? stopStream() : send()"
              />
            </div>
          </div>
        </div>

        <ThoughtSidebar
          :steps="thoughtSteps"
          :is-open="thoughtSidebarOpen"
          :status="thoughtStatus"
          :duration-ms="lastTurnDurationMs"
          @toggle="toggleThoughtSidebar"
        />
      </div>
    </div>

    <q-dialog
      v-model="sessionDrawerOpen"
      position="left"
      full-height
    >
      <q-card class="chat-session-dialog-card column no-wrap full-height" style="width: min(300px, 86vw)">
        <q-card-section class="row items-center q-py-sm">
          <span class="text-subtitle1 text-weight-medium">{{ t('chatSessionHistory') }}</span>
          <q-space />
          <q-btn flat round dense icon="close" :aria-label="t('cancel')" @click="sessionDrawerOpen = false" />
        </q-card-section>
        <q-separator />
        <q-card-section class="col scroll q-pa-none">
          <q-list padding dense class="chat-session-grouped-list">
            <q-inner-loading :showing="sessionsLoading" />
            <div v-if="!sessionsLoading && sessionsList.length === 0" class="text-caption text-text3 q-pa-md">
              {{ t('chatNoSessions') }}
            </div>
            <template v-for="(block, bIdx) in sessionGroupBlocks" :key="'dlg-' + block.key">
              <q-item-label
                header
                :class="['chat-session-group-label', 'text-caption', 'text-text3', bIdx === 0 ? 'chat-session-group-label--first' : '']"
              >
                {{ block.label }}
              </q-item-label>
              <q-item
                v-for="s in block.items"
                :key="'dlg-' + block.key + '-' + s.session_id"
                :active="sessionId === s.session_id"
                active-class="chat-session-item-active"
                class="chat-session-row"
                tabindex="-1"
              >
                <q-item-section clickable v-ripple class="col min-width-0" @click="selectSession(s.session_id)">
                  <q-item-label class="ellipsis">{{ sessionTitle(s) }}</q-item-label>
                  <q-item-label caption class="ellipsis">{{ formatSessionTime(s.updated_at) }}</q-item-label>
                </q-item-section>
                <q-item-section side class="chat-session-row-actions">
                  <div class="row items-center no-wrap">
                    <q-btn
                      flat
                      round
                      dense
                      size="sm"
                      icon="edit"
                      color="grey-7"
                      class="chat-session-row-edit"
                      :aria-label="t('rename')"
                      @click.stop="promptRenameSession(s)"
                    />
                    <q-btn
                      flat
                      round
                      dense
                      size="sm"
                      icon="delete"
                      color="negative"
                      class="chat-session-row-delete"
                      :aria-label="t('delete')"
                      @click.stop="confirmDeleteSession(s)"
                    />
                  </div>
                </q-item-section>
              </q-item>
            </template>
          </q-list>
        </q-card-section>
      </q-card>
    </q-dialog>

    <q-dialog
      v-model="sessionFullDialogOpen"
      transition-show="scale"
      transition-hide="scale"
      full-width
      :maximized="$q.screen.lt.sm"
    >
      <div class="chat-session-dialog-shell">
        <q-card
          class="chat-session-full-dialog column full-width"
          style="max-height: calc(100vh - 100px)"
        >
          <q-card-section class="row items-center q-py-sm q-px-md">
            <span class="text-subtitle1 text-weight-medium">{{ t('chatSessionHistory') }}</span>
            <q-space />
            <q-btn
              v-if="browseSelectedSessionId"
              outline
              dense
              no-caps
              color="primary"
              class="q-mr-xs"
              :label="t('chatBrowseOpenInChat')"
              @click="openBrowseSessionInChat"
            />
            <q-btn flat round dense icon="close" :aria-label="t('cancel')" @click="sessionFullDialogOpen = false" />
          </q-card-section>
          <q-separator />
          <q-card-section class="q-pa-none col column" style="min-height: 0">
            <div class="row col chat-session-browser-body">
              <div class="col-12 col-sm-4 col-md-3 chat-session-browser-left column no-wrap">
                <q-separator />
                <q-scroll-area
                  class="col"
                  style="min-height: 0"
                  @scroll="onSessionBrowseScroll"
                >
                  <q-list padding dense class="chat-session-grouped-list">
                    <q-inner-loading :showing="sessionsBrowseLoading" />
                    <div v-if="!sessionsBrowseLoading && sessionsBrowseList.length === 0" class="text-caption text-text3 q-pa-md">
                      {{ t('chatNoSessions') }}
                    </div>
                    <template v-for="(block, bIdx) in sessionBrowseGroupBlocks" :key="'full-' + block.key">
                      <q-item-label
                        header
                        :class="['chat-session-group-label', 'text-caption', 'text-text3', bIdx === 0 ? 'chat-session-group-label--first' : '']"
                      >
                        {{ block.label }}
                      </q-item-label>
                      <q-item
                        v-for="s in block.items"
                        :key="'full-' + block.key + '-' + s.session_id"
                        :active="browseSelectedSessionId === s.session_id"
                        active-class="chat-session-item-active"
                        class="chat-session-row"
                        tabindex="-1"
                      >
                        <q-item-section clickable v-ripple class="col min-width-0" @click="selectBrowseSession(s.session_id)">
                          <q-item-label class="ellipsis">{{ sessionTitle(s) }}</q-item-label>
                          <q-item-label caption class="ellipsis">{{ formatSessionTime(s.updated_at) }}</q-item-label>
                        </q-item-section>
                        <q-item-section side class="chat-session-row-actions">
                          <div class="row items-center no-wrap">
                            <q-btn
                              flat
                              round
                              dense
                              size="sm"
                              icon="edit"
                              color="grey-7"
                              class="chat-session-row-edit"
                              :aria-label="t('rename')"
                              @click.stop="promptRenameSession(s)"
                            />
                            <q-btn
                              flat
                              round
                              dense
                              size="sm"
                              icon="delete"
                              color="negative"
                              class="chat-session-row-delete"
                              :aria-label="t('delete')"
                              @click.stop="confirmDeleteSession(s)"
                            />
                          </div>
                        </q-item-section>
                      </q-item>
                    </template>
                  </q-list>
                  <div v-if="sessionsBrowseLoadingMore" class="row justify-center q-py-md">
                    <q-spinner-dots color="primary" size="36px" />
                  </div>
                </q-scroll-area>
              </div>
              <q-separator v-if="$q.screen.lt.sm" />
              <div class="col-12 col-sm-8 col-md-9 chat-session-browser-right column no-wrap">
                <q-scroll-area class="col" style="min-height: 0">
                  <q-inner-loading :showing="browseMessagesLoading" />
                  <div v-if="!browseSelectedSessionId" class="text-body2 text-text3 q-pa-xl text-center">
                    {{ t('chatBrowseSelectSession') }}
                  </div>
                  <div v-else class="chat-messages q-px-md q-py-md">
                    <div v-if="browseMessages.length === 0 && !browseMessagesLoading" class="text-center text-text3 q-py-xl">
                      {{ t('chatEmptyThread') }}
                    </div>
                    <div v-for="(m, idx) in browseMessagesForDisplay" :key="'browse-' + idx" class="chat-message-turn">
                      <div
                        v-if="messageDateDividerAt(browseMessagesForDisplay, idx)"
                        class="row justify-center q-mb-sm q-mt-xs"
                      >
                        <span class="text-caption text-text3 chat-date-divider">{{ messageDateDividerAt(browseMessagesForDisplay, idx) }}</span>
                      </div>
                      <div
                        class="chat-message-row row no-wrap q-mb-md"
                        :class="m.role === 'user' ? 'justify-end' : 'justify-start'"
                      >
                        <div
                          class="column chat-bubble-column"
                          :class="[
                            m.role === 'user' ? 'items-end chat-bubble-column-user' : 'items-start chat-bubble-column-assistant'
                          ]"
                        >
                          <div v-if="chatMessageTimeLabel(m)" class="text-caption text-text3 q-mb-xs chat-msg-time">
                            {{ chatMessageTimeLabel(m) }}
                          </div>
                          <div :class="['chat-bubble', m.role === 'user' ? 'chat-bubble-user' : 'chat-bubble-assistant']">
                            <div class="text-caption text-text3 q-mb-xs">
                              {{ m.role === 'user' ? t('chatRoleUser') : t('chatRoleAgent') }}
                            </div>
                            <div
                              v-if="m.role === 'user' && userMessageImageUrls(m).length > 0"
                              class="chat-bubble-user-images row wrap q-gutter-xs q-mb-sm"
                            >
                              <div
                                v-for="(u, imgIdx) in userMessageImageUrls(m)"
                                :key="'browse-img-' + imgIdx"
                                class="chat-bubble-img-wrap"
                              >
                                <img
                                  :src="resolveChatImageUrl(u)"
                                  class="chat-bubble-img-display chat-bubble-img--zoom"
                                  alt=""
                                  loading="eager"
                                  :title="t('chatImageClickHint')"
                                  @click="openImagePreview(u)"
                                >
                              </div>
                            </div>
                            <div
                              v-if="m.role === 'user' && m.file_urls && m.file_urls.length > 0"
                              class="chat-bubble-files q-mb-sm"
                            >
                              <div
                                v-for="(url, fIdx) in m.file_urls"
                                :key="'browse-f-' + fIdx"
                                class="chat-bubble-file row items-center q-mb-xs cursor-pointer"
                                @click="openFilePreview(resolveChatImageUrl(url))"
                              >
                                <q-icon :name="getFileIcon(url)" size="sm" color="primary" class="q-mr-xs" />
                                <span class="text-body2 ellipsis">{{ getFileName(url) }}</span>
                              </div>
                            </div>
                            <div v-if="m.content || (m.reactSteps && m.reactSteps.length > 0)" class="chat-bubble-body text-body2">
                              <template v-if="m.role === 'user'">{{ m.content }}</template>
                              <MdRenderer
                                v-else-if="m.content"
                                :content="m.content"
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </q-scroll-area>
              </div>
            </div>
          </q-card-section>
        </q-card>
      </div>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { useQuasar } from 'quasar'
import { useChatPage } from './useChatPage'
import ThoughtSidebar from 'components/ThoughtSidebar.vue'
import PlanExecutePanel from 'components/PlanExecutePanel.vue'
import MdRenderer from 'components/MdRenderer.vue'
const $q = useQuasar()

defineOptions({
  name: 'ChatPage'
})

function chatPageStyleFn (offset: number, height: number): Record<string, string> {
  const h = height - offset
  return {
    minHeight: `${h}px`,
    maxHeight: `${h}px`,
    height: `${h}px`,
    overflow: 'hidden'
  }
}

const {
  t,
  agents,
  agentsLoading,
  agentId,
  agentOptions,
  sessionId,
  sessionBusy,
  sessionsList,
  sessionsLoading,
  sessionDrawerOpen,
  sessionFullDialogOpen,
  createSessionFromSidebar,
  selectSession,
  formatSessionTime,
  chatMessageTimeLabel,
  messageDateDividerAt,
  sessionTitle,
  promptRenameSession,
  confirmDeleteSession,
  showSessionRail,
  sessionRailCollapsed,
  sessionGroupBlocks,
  sessionRailPreviewBlocks,
  showViewAllSessions,
  sessionBrowseGroupBlocks,
  sessionsBrowseList,
  sessionsBrowseLoading,
  sessionsBrowseLoadingMore,
  browseSelectedSessionId,
  browseMessages,
  browseMessagesForDisplay,
  browseMessagesLoading,
  onSessionBrowseScroll,
  selectBrowseSession,
  openBrowseSessionInChat,
  toggleSessionRailCollapse,
  displayMessages,
  sending,
  stopping,
  draft,
  composerInputRef,
  onComposerKeydown,
  chatScrollRef,
  fillDraft,
  pendingImages,
  imageInputRef,
  clearPendingImages,
  removePendingImageAt,
  onImageSelected,
  onComposerPaste,
  onComposerDragOver,
  onComposerDrop,
  chatImageInputAccept,
  chatDocumentInputAccept,
  canSend,
  canStartSession,
  send,
  stopStream,
  thoughtSidebarOpen,
  thoughtSteps,
  thoughtStatus,
  toggleThoughtSidebar,
  lastTurnDurationMs,
  isAssistantTypingSlot,
  getCurrentPlanTasks,
  getPlanExecuteTasksFromMessage,
  shouldRenderPlanExecuteForMessage,
  shouldHideAssistantPlanMessage,
  shouldHideAssistantMessageText,
  pendingFiles,
  fileInputRef,
  clearPendingFiles,
  removePendingFile,
  formatPendingFileSize,
  onFileSelected,
  getFileIcon,
  getFileName,
  resolveChatImageUrl,
  userMessageImageUrls,
  userMessageTextToDisplay,
  openImagePreview,
  openFilePreview
} = useChatPage()
</script>

<style scoped>
.chat-session-row-edit,
.chat-session-row-delete {
  opacity: 0;
  transition: opacity 0.15s ease;
}
.chat-session-row:hover .chat-session-row-edit,
.chat-session-row:hover .chat-session-row-delete,
.chat-session-row:focus-within .chat-session-row-edit,
.chat-session-row:focus-within .chat-session-row-delete {
  opacity: 1;
}
@media (hover: none) {
  .chat-session-row-edit,
  .chat-session-row-delete {
    opacity: 0.65;
  }
}

.chat-session-dialog-shell {
  flex-shrink: 0;
  width: min(1180px, calc(100vw - 140px)) !important;
  max-width: min(1180px, calc(100vw - 140px)) !important;
  margin-left: auto !important;
  margin-right: auto !important;
  box-sizing: border-box;
}
@media (max-width: 599px) {
  .chat-session-dialog-shell {
    width: 100% !important;
    max-width: 100% !important;
    margin-left: 0 !important;
    margin-right: 0 !important;
  }
}

.chat-session-browser-body {
  flex: 1 1 auto;
  min-height: 360px;
  height: min(78vh, 820px);
}
@media (min-width: 600px) {
  .chat-session-browser-left {
    border-right: 1px solid rgba(0, 0, 0, 0.08);
  }
}
.body--dark .chat-session-browser-left {
  border-right-color: rgba(255, 255, 255, 0.12);
}
@media (max-width: 599px) {
  .chat-session-browser-left {
    min-height: 200px;
    max-height: 50vh;
  }
}
</style>
