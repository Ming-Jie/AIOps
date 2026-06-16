import { computed, nextTick, onMounted, onUnmounted, ref, watch, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { LocalStorage, useQuasar } from 'quasar'
import ChatImagePreviewDialog from 'src/components/ChatImagePreviewDialog.vue'
import { api } from 'boot/axios'
import { isCancel } from 'axios'
import type { Agent, APIResponse, ChatHistoryMessage, ChatReactStep, ChatSession, Skill } from 'src/api/types'
import { hydrateReactStepsFromServer } from 'src/utils/reactStepsHydrate'
import { buildSkillRiskLookup, resolveClientToolRiskLevel } from 'src/utils/toolRisk'
import { onChatInputEnterToSend } from 'src/utils/chatComposer'
import { logChatAttach } from './chat/chatAttachDebug'
import {
  CHAT_DOCUMENT_INPUT_ACCEPT,
  CHAT_IMAGE_INPUT_ACCEPT,
  inferImageMimeFromFile,
  isAllowedDocumentButtonFile,
  isAllowedImageButtonFile
} from './chat/chatAttachmentRules'
import {
  getFileIcon,
  getFileName,
  isSafeImagePreviewSrc,
  resolveChatImageUrl,
  userMessageImageUrls,
  userMessageTextToDisplay as userMessageTextToDisplayI18n
} from './chat/chatMessageDisplay'
import {
  findAgentFromRouteParam,
  LAST_AGENT_PUBLIC_ID_KEY
} from './chat/chatRouteHelpers'
import {
  SESSION_BROWSE_PAGE_SIZE,
  SESSION_LIST_FETCH_LIMIT,
  SESSION_MESSAGES_PAGE_SIZE,
  SESSION_RAIL_PREVIEW_MAX
} from './chat/chatSessionConstants'
import {
  chatDateDividerText,
  chatMessageTimeLabel,
  dayKeyLocal,
  formatChatMessageTime,
  formatSessionTime,
  messageDateDividerAt,
  sessionTitle
} from './chat/chatSessionTime'
import { formatPendingFileSize } from './chat/chatFormatBytes'
import {
  ROUTE_SESSION_Q,
  isLikelySessionUUID,
  normalizeRouteQuery
} from './chat/chatRouteQuery'
import { chatReactStepsToThoughtSteps } from './chat/chatReactStepsSync'
import {
  initTasksFromPlanTasksPayload,
  mergePlanDetailFromReActPayload,
  type PlanTaskRowWeb
} from 'src/utils/planExecuteMerge'
import {
  processReActEvent as processReActEventImpl,
  processStreamEvent as processStreamEventImpl
} from './chat/chatReactStreamProcessors'
import { createChatSSEParser } from './chat/chatParseSSE'
import { createSendStreamLifecycle, type ChatSendStreamLocalTurnRow } from './chat/chatSendStreamRuntime'
import {
  computeStreamMessageLabel,
  uploadPendingChatAttachments
} from './chat/chatSendAttachments'
import {
  handleComposerDragOver,
  handleComposerDrop,
  handleComposerPaste
} from './chat/chatComposerAttach'

const THOUGHT_STEPS_MAX = 200

function createChatPageState () {
  const { t } = useI18n()
  const $q = useQuasar()
  const route = useRoute()
  const router = useRouter()

  const agentsLoading = ref(false)

  const agents = ref<Agent[]>([])
  const agentId = ref<number | null>(null)

  const agentOptions = computed(() =>
    agents.value.map(a => ({
      id: a.id,
      label: a.public_id ? `${a.name}` : `${a.name} (#${a.id})`
    }))
  )

  const canStartSession = computed(() => agentId.value != null && agentId.value >= 1)

  const sessionId = ref<string | null>(null)
  const sessionBusy = ref(false)
  const sessionsList = ref<ChatSession[]>([])
  const sessionsLoading = ref(false)
  const suppressSessionListAutoPick = ref(false)
  const sessionDrawerOpen = ref(false)
  const SESSION_RAIL_LEGACY = 'aiops_chat_session_rail_collapsed'
  const SESSION_RAIL_COLLAPSE_KEY = 'aitaskmeta_chat_session_rail_collapsed'

  const sessionRailCollapsed = ref((() => {
    let v = LocalStorage.getItem(SESSION_RAIL_COLLAPSE_KEY)
    if (v !== '1' && v !== '0') v = LocalStorage.getItem(SESSION_RAIL_LEGACY)
    if (v === '1' || v === '0') {
      LocalStorage.set(SESSION_RAIL_COLLAPSE_KEY, v)
      LocalStorage.removeItem(SESSION_RAIL_LEGACY)
    }
    return v === '1'
  })())

  function routePathSegment (): string {
    const raw = route.params.agentId
    return typeof raw === 'string' ? raw : Array.isArray(raw) ? raw[0] ?? '' : ''
  }

  const history = ref<ChatHistoryMessage[]>([])
  const localTurns = ref<
    {
      role: string
      content: string
      displayedContent: string
      duration?: number
      image_urls?: string[]
      image_data_urls?: string[]
      file_urls?: string[]
      createdAt?: string
      reactSteps?: ChatReactStep[]
      agentId?: number
    }[]
  >([])

  const displayMessages = computed(() => {
    const fromHistory = history.value.map(h => ({
      role: h.role,
      content: h.content,
      displayedContent: h.content,
      agentId: h.agent_id != null && h.agent_id >= 1 ? h.agent_id : undefined,
      duration: undefined as number | undefined,
      image_urls: h.image_urls ?? [],
      image_data_urls: undefined as string[] | undefined,
      file_urls: h.file_urls ?? [],
      createdAt: h.created_at,
      reactSteps: hydrateReactStepsFromServer(h.react_steps)
    }))
    return [...fromHistory, ...localTurns.value]
  })

  const maxChatImages = 12
  const pendingImages = ref<{ dataUrl: string; base64: string; mime: string }[]>([])
  const pendingFiles = ref<{ name: string; size: number; base64: string; mime: string; url?: string }[]>([])
  const imageInputRef = ref<HTMLInputElement | null>(null)
  const fileInputRef = ref<HTMLInputElement | null>(null)

  const maxChatImageBytes = 8 * 1024 * 1024

  const sending = ref(false)
  const stopping = ref(false)
  let streamAbortController: AbortController | null = null
  const draft = ref('')
  const composerInputRef = ref<{ $el?: HTMLElement } | null>(null)
  const chatScrollRef = ref<HTMLElement | null>(null)

  watch(
    () => [pendingImages.value.length, pendingFiles.value.length] as const,
    async ([ni, nf]) => {
      logChatAttach(`pending counts images=${ni} files=${nf}`)
      await nextTick()
      const strip =
        typeof document !== 'undefined' ? document.querySelector('.chat-pending-strip') : null
      const h = strip instanceof HTMLElement ? strip.offsetHeight : null
      const disp = strip instanceof HTMLElement ? window.getComputedStyle(strip).display : null
      logChatAttach(
        `DOM after tick scrollRef=${!!chatScrollRef.value} strip=${!!strip} stripH=${h} stripDisplay=${disp}`
      )
    }
  )

  const currentStreamModelName = ref('')

  const skillRiskLookup = ref<Record<string, string>>({})

  async function loadSkillRiskLookup (): Promise<void> {
    try {
      const { data } = await api.get<APIResponse<Skill[]>>('/skills')
      if (data.code === 0 && Array.isArray(data.data)) {
        skillRiskLookup.value = buildSkillRiskLookup(data.data)
      }
    } catch {
      /* 保持已有 lookup，避免刷屏 */
    }
  }

  function resolveToolRiskForStream (toolName: string, payloadRisk: unknown): string {
    return resolveClientToolRiskLevel(toolName, payloadRisk, skillRiskLookup.value)
  }

  const THOUGHT_SIDEBAR_LEGACY = 'aiops_chat_thought_sidebar_open'
  const THOUGHT_SIDEBAR_KEY = 'aitaskmeta_chat_thought_sidebar_open'
  const thoughtSidebarOpen = ref((() => {
    let v = LocalStorage.getItem(THOUGHT_SIDEBAR_KEY)
    if (v !== '1' && v !== '0') v = LocalStorage.getItem(THOUGHT_SIDEBAR_LEGACY)
    if (v === '1' || v === '0') {
      LocalStorage.set(THOUGHT_SIDEBAR_KEY, v)
      LocalStorage.removeItem(THOUGHT_SIDEBAR_LEGACY)
    }
    return v === '1'
  })())
  const thoughtSteps = ref<{ type: string; data: Record<string, unknown>; meta?: Record<string, unknown>; timestamp?: string }[]>([])
  const lastServerToolResultText = { value: '' }
  const thoughtStatus = ref<'running' | 'completed'>('completed')
  const lastTurnDurationMs = ref<number | null>(null)

  function toggleThoughtSidebar (): void {
    thoughtSidebarOpen.value = !thoughtSidebarOpen.value
    LocalStorage.set(THOUGHT_SIDEBAR_KEY, thoughtSidebarOpen.value ? '1' : '0')
    LocalStorage.removeItem(THOUGHT_SIDEBAR_LEGACY)
  }

  function pushThoughtStep (step: { type: string; data: Record<string, unknown>; meta?: Record<string, unknown>; timestamp?: string }): void {
    if (!step || !step.type || !step.data) return
    thoughtSteps.value.push({
      ...step,
      timestamp: step.timestamp || new Date().toISOString()
    })
    if (thoughtSteps.value.length > THOUGHT_STEPS_MAX) {
      thoughtSteps.value.splice(0, thoughtSteps.value.length - THOUGHT_STEPS_MAX)
    }
  }

  function upsertPlanExecute (payload: Record<string, unknown>): void {
    const reactType = payload.type as string
    if (reactType === 'plan_tasks') {
      const tasks = initTasksFromPlanTasksPayload(payload)
      if (tasks.length === 0) return
      thoughtSteps.value = thoughtSteps.value.filter(
        (s) => !(s.type === 'plan' && s.data?.kind === 'plan_execute')
      )
      pushThoughtStep({
        type: 'plan',
        data: { kind: 'plan_execute', tasks }
      })
      return
    }
    if (reactType === 'plan_step') {
      const stepNum = payload.step as number | undefined
      const st = payload.plan_step_status as string | undefined
      if (stepNum == null || !st) return
      const list = thoughtSteps.value
      for (let i = list.length - 1; i >= 0; i--) {
        const s = list[i]
        if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
          const tasks = (s.data.tasks as PlanTaskRowWeb[]) || []
          const idx = stepNum - 1
          if (idx < 0 || idx >= tasks.length) return
          const next = [...tasks]
          const cur = next[idx]
          let nextStatus: 'pending' | 'running' | 'done' | 'error' = cur.status
          if (st === 'running') nextStatus = 'running'
          else if (st === 'done') nextStatus = 'done'
          else if (st === 'error') nextStatus = 'error'
          next[idx] = { ...cur, status: nextStatus }
          list[i] = { ...s, data: { ...s.data, tasks: next } }
          thoughtSteps.value = [...list]
          return
        }
      }
    }
  }

  function mergePlanExecuteReActEvent (payload: Record<string, unknown>): boolean {
    const list = thoughtSteps.value
    for (let i = list.length - 1; i >= 0; i--) {
      const s = list[i]
      if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
        const tasks = (s.data.tasks as PlanTaskRowWeb[]) || []
        const merged = mergePlanDetailFromReActPayload(tasks, payload)
        if (!merged) return false
        list[i] = { ...s, data: { ...s.data, tasks: merged } }
        thoughtSteps.value = [...list]
        return true
      }
    }
    return false
  }

  function clearThoughtSteps (): void {
    thoughtSteps.value = []
    lastServerToolResultText.value = ''
    thoughtStatus.value = 'completed'
  }

  function finalizeRunningPlanTasksOnStop (): void {
    const list = thoughtSteps.value
    for (let i = list.length - 1; i >= 0; i--) {
      const s = list[i]
      if (s.type !== 'plan' || s.data?.kind !== 'plan_execute') continue
      const tasks = ((s.data.tasks as PlanTaskRowWeb[]) || []).map((row) => {
        if (row.status !== 'running') return row
        const details = row.details ?? []
        const stopText = '已手动停止回复'
        const nextDetails = details.some(d => d.text === stopText)
          ? details
          : [...details, { text: stopText, tone: 'muted' as const }]
        return { ...row, status: 'pending' as const, details: nextDetails }
      })
      list[i] = { ...s, data: { ...s.data, tasks } }
      thoughtSteps.value = [...list]
      return
    }
  }

  function syncThoughtSidebarFromLoadedHistory (): void {
    if (sending.value) return
    const msgs = displayMessages.value
    for (let i = msgs.length - 1; i >= 0; i--) {
      const m = msgs[i] as { role: string; reactSteps?: ChatReactStep[] }
      if (m.role !== 'assistant') continue
      const rs = m.reactSteps
      if (rs != null && rs.length > 0) {
        thoughtSteps.value = chatReactStepsToThoughtSteps(rs)
        thoughtStatus.value = 'completed'
        return
      }
    }
    thoughtSteps.value = []
    thoughtStatus.value = 'completed'
  }

  function processReActEvent (payload: Record<string, unknown>): void {
    processReActEventImpl(payload, {
      pushThoughtStep,
      resolveToolRiskForStream,
      upsertPlanExecute,
      mergePlanExecuteReActEvent
    })
  }

  function processStreamEvent (payload: Record<string, unknown>): void {
    processStreamEventImpl(payload, {
      pushThoughtStep,
      resolveToolRiskForStream,
      setLastServerToolResult: (t: string) => {
        lastServerToolResultText.value = t
      }
    })
  }

  function scrollChatToBottom (): void {
    void nextTick(() => {
      const el = chatScrollRef.value
      if (!el) return
      // 流式传输中用 auto 避免 smooth 动画与内容增长互相拉扯导致抖动
      el.scrollTo({ top: el.scrollHeight, behavior: sending.value ? 'auto' : 'smooth' })
    })
  }

  function fillDraft (text: string): void {
    draft.value = text
  }

  function clearPendingImages (): void {
    pendingImages.value = []
  }

  function removePendingImageAt (idx: number): void {
    pendingImages.value.splice(idx, 1)
  }

  function enqueuePendingDocumentFromFile (file: File): void {
    const maxFileSize = 10 * 1024 * 1024
    if (!isAllowedDocumentButtonFile(file)) {
      $q.notify({ type: 'warning', message: t('chatDocumentTypeNotAllowed') })
      return
    }
    if (file.size > maxFileSize) {
      $q.notify({ type: 'warning', message: t('chatFileTooLarge') })
      return
    }
    const reader = new FileReader()
    reader.onerror = () => {
      $q.notify({ type: 'negative', message: `${t('chatAttachFile')}: 读取失败` })
    }
    reader.onload = () => {
      const dataUrl = reader.result as string
      const comma = dataUrl.indexOf(',')
      if (comma < 0) return
      const base64 = dataUrl.slice(comma + 1)
      const rawMime = (file.type || '').trim()
      const lower = file.name.toLowerCase()
      let fm = rawMime
      if (!fm) {
        if (lower.endsWith('.pdf')) fm = 'application/pdf'
        else if (lower.endsWith('.json')) fm = 'application/json'
        else if (lower.endsWith('.md')) fm = 'text/markdown'
        else fm = 'text/plain'
      }
      pendingFiles.value = [
        ...pendingFiles.value,
        { name: file.name, size: file.size, base64, mime: fm }
      ]
      logChatAttach('pending document', { name: file.name, mime: fm })
      void nextTick(() => scrollChatToBottom())
    }
    reader.readAsDataURL(file)
  }

  function onFileSelected (e: Event): void {
    const input = e.target as HTMLInputElement
    const fileArray = input.files?.length ? Array.from(input.files) : []
    input.value = ''
    logChatAttach(
      `onFileSelected files=${fileArray.length} canStart=${canStartSession.value} agentId=${agentId.value} inputDisabled=${input.disabled}`
    )
    if (fileArray.length === 0) return
    for (const file of fileArray) {
      enqueuePendingDocumentFromFile(file)
    }
  }

  function clearPendingFiles (): void {
    pendingFiles.value = []
  }

  function removePendingFile (idx: number): void {
    pendingFiles.value.splice(idx, 1)
  }

  async function uploadFile (file: File): Promise<{ url: string; filename: string } | null> {
    const formData = new FormData()
    formData.append('file', file)
    try {
      const { data } = await api.post<APIResponse<{ url: string; filename: string }>>('/chat/upload', formData, {
        headers: { 'Content-Type': 'multipart/form-data' }
      })
      if (data.code === 0 && data.data) {
        return data.data
      }
      $q.notify({ type: 'negative', message: data.message || '上传失败' })
      return null
    } catch (e) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '上传失败' })
      return null
    }
  }

  function setPendingImageFromFile (file: File): void {
    const mime = file.type || ''
    logChatAttach(
      `setPendingImageFromFile enter name=${file.name} size=${file.size} mime=${mime || '(empty)'}`
    )
    if (!isAllowedImageButtonFile(file)) {
      logChatAttach('setPendingImageFromFile abort: type not allowed')
      $q.notify({ type: 'warning', message: t('chatImageTypeNotAllowed') })
      return
    }
    if (file.size > maxChatImageBytes) {
      logChatAttach(`setPendingImageFromFile abort: file too large (>${maxChatImageBytes})`)
      $q.notify({ type: 'warning', message: t('chatImageTooLarge') })
      return
    }
    const reader = new FileReader()
    reader.onerror = () => {
      logChatAttach('setPendingImageFromFile FileReader.onerror', reader.error?.message ?? 'unknown')
      $q.notify({ type: 'negative', message: `${t('chatAttachImage')}: 读取失败` })
    }
    reader.onload = () => {
      const dataUrl = reader.result as string
      const comma = dataUrl.indexOf(',')
      if (comma < 0) {
        logChatAttach('setPendingImageFromFile: invalid dataUrl (no comma)')
        return
      }
      const base64 = dataUrl.slice(comma + 1)
      if (pendingImages.value.length >= maxChatImages) {
        $q.notify({ type: 'warning', message: t('chatMaxImages', { n: maxChatImages }) })
        return
      }
      pendingImages.value = [...pendingImages.value, {
        dataUrl,
        base64,
        mime: inferImageMimeFromFile(file)
      }]
      logChatAttach(
        `setPendingImageFromFile ok name=${file.name} mime=${mime || 'image/png'} dataUrlLen=${dataUrl.length} prefix=${dataUrl.slice(0, 40)} pendingTotal=${pendingImages.value.length}`
      )
      void nextTick(() => scrollChatToBottom())
    }
    logChatAttach('setPendingImageFromFile FileReader.readAsDataURL(...) start')
    reader.readAsDataURL(file)
  }

  function onImageSelected (e: Event): void {
    const input = e.target as HTMLInputElement
    const picked = input.files?.length ? Array.from(input.files) : []
    input.value = ''
    logChatAttach(
      `onImageSelected files=${picked.length} canStart=${canStartSession.value} agentId=${agentId.value} inputDisabled=${input.disabled}`
    )
    if (picked.length === 0) return
    for (let i = 0; i < picked.length; i++) {
      if (pendingImages.value.length >= maxChatImages) {
        $q.notify({ type: 'warning', message: t('chatMaxImages', { n: maxChatImages }) })
        break
      }
      setPendingImageFromFile(picked[i])
    }
  }

  const composerAttachDeps = () => ({
    canStartSession: () => canStartSession.value,
    maxChatImages,
    maxChatImageBytes,
    getPendingImageCount: () => pendingImages.value.length,
    t,
    logChatAttach,
    notifyWarning: (message: string) => { $q.notify({ type: 'warning', message }) },
    setPendingImageFromFile,
    enqueuePendingDocumentFromFile
  })

  function onComposerPaste (e: ClipboardEvent): void {
    handleComposerPaste(e, composerAttachDeps())
  }

  function onComposerDragOver (e: DragEvent): void {
    handleComposerDragOver(e, composerAttachDeps())
  }

  function onComposerDrop (e: DragEvent): void {
    handleComposerDrop(e, composerAttachDeps())
  }

  function syncAgentIdToRoute (): void {
    const id = agentId.value
    const curRaw = route.params.agentId
    const cur = typeof curRaw === 'string' ? curRaw : Array.isArray(curRaw) ? (curRaw[0] ?? '') : ''
    if (id != null && id >= 1) {
      const ag = agents.value.find(a => a.id === id)
      const pub = (ag?.public_id ?? '').trim()
      const want = pub !== '' ? pub : String(id)
      const sid = (sessionId.value ?? '').trim()
      const query = sid !== '' ? { [ROUTE_SESSION_Q]: sid } : {}
      const curSid = normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)
      if (cur === want && curSid === sid) {
        return
      }
      void router.replace({ name: 'chat', params: { agentId: want }, query })
    }
  }

  async function resolveSessionFromAPI (sid: string): Promise<ChatSession | null> {
    try {
      const { data } = await api.get<APIResponse<ChatSession>>(
        `/chat/sessions/${encodeURIComponent(sid)}`
      )
      return (data.data as ChatSession) ?? null
    } catch {
      return null
    }
  }

  async function openSessionById (sid: string): Promise<boolean> {
    const sess = await resolveSessionFromAPI(sid)
    if (!sess?.session_id) return false
    suppressSessionListAutoPick.value = true
    try {
      sessionId.value = sess.session_id
      localTurns.value = []
      lastTurnDurationMs.value = null
      if (sess.agent_id >= 1) {
        agentId.value = sess.agent_id
        const ag = agents.value.find(a => a.id === sess.agent_id)
        const pub = (ag?.public_id ?? '').trim() || String(sess.agent_id)
        void router.replace({
          name: 'chat',
          params: { agentId: pub },
          query: { [ROUTE_SESSION_Q]: sess.session_id }
        })
      }
      await loadMessages()
      return true
    } finally {
      suppressSessionListAutoPick.value = false
    }
  }

  async function loadAgents () {
    agentsLoading.value = true
    try {
      const { data } = await api.get<APIResponse<Agent[]>>('/agents')
      agents.value = (data.data ?? []) as Agent[]
      const pathStr = routePathSegment()
      const looksLikeSessionUuidPath = pathStr !== '' && isLikelySessionUUID(pathStr)
      const querySid = normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)
      const hasSessionInQuery = querySid !== ''
      const fromStoragePub = LocalStorage.getItem<string>(LAST_AGENT_PUBLIC_ID_KEY)
      const legacyNum = LocalStorage.getItem<string>('lastAgentId')
      if (hasSessionInQuery) {
        const sess = await resolveSessionFromAPI(querySid)
        const aid = sess?.agent_id
        if (sess && aid != null && aid >= 1) {
          agentId.value = aid
          sessionId.value = querySid
          await loadMessages()
        }
      } else {
        const raw =
          pathStr !== '' ? pathStr : (fromStoragePub ?? '')
        let picked = findAgentFromRouteParam(raw, agents.value)
        if (!picked && (fromStoragePub == null || fromStoragePub === '') && legacyNum != null && legacyNum !== '') {
          picked = findAgentFromRouteParam(legacyNum, agents.value)
        }
        if (picked) {
          agentId.value = picked.id
          if ((picked.public_id || '').trim() !== '') {
            const routeStr = pathStr
            if (routeStr !== picked.public_id && /^\d+$/.test(routeStr.trim())) {
              void router.replace({ name: 'chat', params: { agentId: picked.public_id }, query: {} })
            }
          }
        } else if (looksLikeSessionUuidPath) {
          if (await openSessionById(pathStr)) {
            return
          }
          const stored = fromStoragePub
            ? findAgentFromRouteParam(fromStoragePub, agents.value)
            : undefined
          agentId.value = stored?.id ?? (agents.value.length > 0 ? agents.value[0].id : null)
          if (agentId.value != null && agentId.value >= 1) {
            const ag = agents.value.find(a => a.id === agentId.value)
            const pub = (ag?.public_id ?? '').trim()
            void router.replace({ name: 'chat', params: { agentId: pub || String(agentId.value) }, query: {} })
          } else {
            void router.replace({ name: 'chat', params: {}, query: {} })
          }
        } else if (agents.value.length > 0) {
          agentId.value = agents.value[0].id
        } else {
          agentId.value = null
        }
      }
    } finally {
      agentsLoading.value = false
    }
  }

  const canSend = computed(() => {
    if (sending.value) return false
    if (!canStartSession.value) return false
    const hasText = draft.value.trim().length > 0
    const hasImg = pendingImages.value.length > 0
    const hasFiles = pendingFiles.value.length > 0
    return hasText || hasImg || hasFiles
  })

  const showSessionRail = computed(() => $q.screen.gt.sm)

  function groupSessionsByDay (sessions: ChatSession[]): Map<string, ChatSession[]> {
    const map = new Map<string, ChatSession[]>()
    for (const s of sessions) {
      const key = dayKeyLocal(s.updated_at)
      if (!key) continue
      if (!map.has(key)) map.set(key, [])
      map.get(key)!.push(s)
    }
    return map
  }

  function sessionBlocksFromGrouped (grouped: Map<string, ChatSession[]>, t: (key: string) => string): { key: string; label: string; items: ChatSession[] }[] {
    const todayKey = dayKeyLocal(new Date().toISOString())
    const yesterdayDate = new Date()
    yesterdayDate.setDate(yesterdayDate.getDate() - 1)
    const yesterdayKey = dayKeyLocal(yesterdayDate.toISOString())
    const out: { key: string; label: string; items: ChatSession[] }[] = []
    for (const [key, items] of grouped) {
      const sortedItems = [...items].sort(
        (a, b) => new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime()
      )
      const label = key === todayKey ? t('chatSessionGroupToday') : key === yesterdayKey ? t('chatSessionGroupYesterday') : t('chatSessionGroupEarlier')
      out.push({ key, label, items: sortedItems })
    }
    return out.sort((a, b) => b.key.localeCompare(a.key))
  }

  const sessionGroupBlocks = computed(() =>
    sessionBlocksFromGrouped(groupSessionsByDay(sessionsList.value), t)
  )

  const sessionsBrowseList = ref<ChatSession[]>([])
  const sessionsBrowseLoading = ref(false)
  const sessionsBrowseLoadingMore = ref(false)
  const sessionsBrowseHasMore = ref(false)
  const sessionsBrowseOffset = ref(0)
  const browseSelectedSessionId = ref<string | null>(null)
  const browseMessages = ref<ChatHistoryMessage[]>([])
  const browseMessagesLoading = ref(false)

  const browseMessagesForDisplay = computed(() =>
    browseMessages.value.map(h => ({
      ...h,
      reactSteps: hydrateReactStepsFromServer(h.react_steps)
    }))
  )

  const sessionBrowseGroupBlocks = computed(() =>
    sessionBlocksFromGrouped(groupSessionsByDay(sessionsBrowseList.value), t)
  )

  let browseScrollDebounce: ReturnType<typeof setTimeout> | null = null

  const sessionRailPreviewBlocks = computed(() => {
    const blocks = sessionGroupBlocks.value
    const out: { key: string; label: string; items: ChatSession[] }[] = []
    let count = 0
    for (const b of blocks) {
      if (count >= SESSION_RAIL_PREVIEW_MAX) break
      const slice = b.items.slice(0, SESSION_RAIL_PREVIEW_MAX - count)
      if (slice.length === 0) continue
      out.push({ ...b, items: slice })
      count += slice.length
    }
    return out
  })

  const showViewAllSessions = computed(() => sessionsList.value.length > SESSION_RAIL_PREVIEW_MAX)

  const sessionFullDialogOpen = ref(false)

  const sessionListModeTab = ref<'single'>('single')

  function toggleSessionRailCollapse (): void {
    sessionRailCollapsed.value = !sessionRailCollapsed.value
    LocalStorage.set(SESSION_RAIL_COLLAPSE_KEY, sessionRailCollapsed.value ? '1' : '0')
    LocalStorage.removeItem(SESSION_RAIL_LEGACY)
  }

  async function fetchSessionsPage (offset: number, limit: number): Promise<ChatSession[]> {
    if (agentId.value == null || agentId.value < 1) return []
    const attempts = 3
    for (let i = 0; i < attempts; i++) {
      try {
        const { data } = await api.get<APIResponse<ChatSession[]>>(
          `/chat/sessions?agent_id=${agentId.value}&limit=${limit}&offset=${offset}`
        )
        return (data.data ?? []) as ChatSession[]
      } catch (e: unknown) {
        const err = e as { response?: { status?: number }; message?: string; code?: string }
        const isCanceled = err.code === 'ERR_CANCELED' || err.message?.includes('canceled') || err.message?.includes('Cancel')
        if (isCanceled && i < attempts - 1) {
          await new Promise(resolve => setTimeout(resolve, 100 * (i + 1)))
          continue
        }
        throw e
      }
    }
    return []
  }

  async function fetchSessionsList (): Promise<ChatSession[]> {
    return fetchSessionsPage(0, SESSION_LIST_FETCH_LIMIT)
  }

  async function fetchAllSessionMessages (sid: string): Promise<ChatHistoryMessage[]> {
    const all: ChatHistoryMessage[] = []
    let offset = 0
    while (true) {
      const { data } = await api.get<APIResponse<ChatHistoryMessage[]>>(
        `/chat/sessions/${encodeURIComponent(sid)}/messages`,
        { params: { limit: SESSION_MESSAGES_PAGE_SIZE, offset } }
      )
      const batch = (data.data ?? []) as ChatHistoryMessage[]
      all.push(...batch)
      if (batch.length < SESSION_MESSAGES_PAGE_SIZE) break
      offset += batch.length
    }
    return all
  }

  async function loadBrowseMessages (sid: string) {
    browseMessagesLoading.value = true
    try {
      browseMessages.value = await fetchAllSessionMessages(sid)
    } catch (e: unknown) {
      if (isCancel(e)) return
      browseMessages.value = []
    } finally {
      browseMessagesLoading.value = false
    }
  }

  async function loadMoreSessionsBrowse () {
    if (!sessionsBrowseHasMore.value || sessionsBrowseLoadingMore.value || agentId.value == null) return
    sessionsBrowseLoadingMore.value = true
    try {
      const batch = await fetchSessionsPage(sessionsBrowseOffset.value, SESSION_BROWSE_PAGE_SIZE)
      const seen = new Set(sessionsBrowseList.value.map(s => s.session_id))
      for (const s of batch) {
        if (!seen.has(s.session_id)) {
          sessionsBrowseList.value.push(s)
          seen.add(s.session_id)
        }
      }
      sessionsBrowseOffset.value += batch.length
      if (batch.length === 0 || batch.length < SESSION_BROWSE_PAGE_SIZE) {
        sessionsBrowseHasMore.value = false
      }
    } finally {
      sessionsBrowseLoadingMore.value = false
    }
  }

  function onSessionBrowseScroll (info: { verticalPercentage: number }) {
    if (info.verticalPercentage < 0.88) return
    if (browseScrollDebounce != null) return
    browseScrollDebounce = setTimeout(() => {
      browseScrollDebounce = null
      void loadMoreSessionsBrowse()
    }, 200)
  }

  async function selectBrowseSession (sid: string) {
    if (browseSelectedSessionId.value === sid) return
    browseSelectedSessionId.value = sid
    await loadBrowseMessages(sid)
  }

  function openBrowseSessionInChat () {
    const sid = browseSelectedSessionId.value
    if (!sid) return
    void selectSession(sid)
  }

  watch(sessionFullDialogOpen, async (open) => {
    if (!open) {
      browseSelectedSessionId.value = null
      browseMessages.value = []
      return
    }
    sessionsBrowseLoading.value = true
    try {
      if (sessionsList.value.length === 0) {
        const batch = await fetchSessionsPage(0, SESSION_LIST_FETCH_LIMIT)
        sessionsBrowseList.value = batch
        sessionsBrowseOffset.value = batch.length
        sessionsBrowseHasMore.value = batch.length >= SESSION_LIST_FETCH_LIMIT
      } else {
        sessionsBrowseList.value = [...sessionsList.value]
        sessionsBrowseOffset.value = sessionsList.value.length
        sessionsBrowseHasMore.value = sessionsList.value.length >= SESSION_LIST_FETCH_LIMIT
      }
      const pick =
        sessionId.value && sessionsBrowseList.value.some(s => s.session_id === sessionId.value)
          ? sessionId.value
          : sessionsBrowseList.value[0]?.session_id ?? null
      browseSelectedSessionId.value = pick
      if (pick) await loadBrowseMessages(pick)
      else browseMessages.value = []
    } finally {
      sessionsBrowseLoading.value = false
    }
  })

  async function loadSessions () {
    if (agentId.value == null || agentId.value < 1) {
      sessionsList.value = []
      sessionId.value = null
      history.value = []
      lastTurnDurationMs.value = null
      return
    }
    sessionsLoading.value = true
    try {
      const list = await fetchSessionsList()
      sessionsList.value = list
      if (list.length === 0) {
        history.value = []
        if (!sending.value && localTurns.value.length === 0) {
          sessionId.value = null
          localTurns.value = []
          lastTurnDurationMs.value = null
        }
        return
      }
      const before = sessionId.value
      const exists = before != null && list.some(s => s.session_id === before)
      if (!exists && list.length > 0 && !suppressSessionListAutoPick.value) {
        sessionId.value = list[0].session_id
        localTurns.value = []
        lastTurnDurationMs.value = null
        await loadMessages()
      }
    } catch (e: unknown) {
      sessionsList.value = []
      sessionId.value = null
      history.value = []
      lastTurnDurationMs.value = null
      const err = e as { response?: { status?: number }; message?: string }
      if (err.response?.status === 503) {
        $q.notify({ type: 'warning', message: '会话服务不可用，请检查数据库配置' })
      }
    } finally {
      sessionsLoading.value = false
    }
  }

  async function refreshSessionsList () {
    if (agentId.value == null || agentId.value < 1) return
    try {
      sessionsList.value = await fetchSessionsList()
    } catch {
      /* ignore */
    }
  }

  async function selectSession (sid: string) {
    if (sessionId.value === sid) {
      sessionDrawerOpen.value = false
      sessionFullDialogOpen.value = false
      return
    }
    sessionListModeTab.value = 'single'
    let sess = sessionsList.value.find(s => s.session_id === sid)
    if (!sess) {
      try {
        const { data } = await api.get<APIResponse<ChatSession>>(
          `/chat/sessions/${encodeURIComponent(sid)}`
        )
        sess = data.data as ChatSession
      } catch {
        /* 无权限或不存在 */
      }
    }
    if (!sess) {
      $q.notify({ type: 'warning', message: t('chatSessionNotFound') })
      void refreshSessionsList()
      return
    }
    suppressSessionListAutoPick.value = true
    try {
      sessionId.value = sid
      localTurns.value = []
      lastTurnDurationMs.value = null
      if (sess.agent_id >= 1) {
        agentId.value = sess.agent_id
      }
      await loadMessages()
    } finally {
      suppressSessionListAutoPick.value = false
    }
    sessionDrawerOpen.value = false
    sessionFullDialogOpen.value = false
    syncAgentIdToRoute()
  }

  async function loadMessages (): Promise<boolean> {
    if (!sessionId.value) return false
    try {
      history.value = await fetchAllSessionMessages(sessionId.value)
      await nextTick()
      syncThoughtSidebarFromLoadedHistory()
      return true
    } catch (e: unknown) {
      if (isCancel(e)) return false
      history.value = []
      const err = e as { response?: { status?: number } }
      if (err.response?.status === 403) {
        $q.notify({ type: 'warning', message: '无权查看该会话，请确认已重启后端并刷新页面' })
      }
      if (err.response?.status === 404) {
        $q.notify({ type: 'warning', message: t('chatSessionNotFound') })
      }
      if (err.response?.status === 503) {
        $q.notify({ type: 'warning', message: '无法加载聊天记录' })
      }
      return false
    }
  }

  async function createSession () {
    if (agentId.value == null || agentId.value < 1) return
    sessionBusy.value = true
    try {
      const { data } = await api.post<APIResponse<ChatSession>>('/chat/sessions', { agent_id: agentId.value })
      const sess = data.data as ChatSession
      sessionId.value = sess.session_id
      localTurns.value = []
      history.value = []
      lastTurnDurationMs.value = null
      clearThoughtSteps()
      await refreshSessionsList()
      $q.notify({ type: 'positive', message: '已创建会话' })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string }; status?: number } }
      if (err.response?.status === 503) {
        $q.notify({ type: 'warning', message: '暂无法创建会话' })
      } else {
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? '创建会话失败' })
      }
    } finally {
      sessionBusy.value = false
    }
  }

  async function createSessionFromSidebar () {
    await createSession()
    syncAgentIdToRoute()
  }

  function onSessionRailAddClick () {
    void createSessionFromSidebar()
  }

  async function sendStream (text: string, fileUrls?: string[], imageUrlsForHistory?: string[]) {
    const baseURL = (api.defaults.baseURL ?? '/api/v1').replace(/\/$/, '')
    const url = `${baseURL}/chat/stream`
    const token = LocalStorage.getItem('access') as string | null

    const assistantIdx = localTurns.value.length
    localTurns.value.push({
      role: 'assistant',
      content: '',
      displayedContent: '',
      createdAt: new Date().toISOString()
    })
    const assistantEntries = new Map<number, number>([[0, assistantIdx]])
    const pendingAgentIds: number[] = []

    thoughtStatus.value = 'running'
    lastTurnDurationMs.value = null
    clearThoughtSteps()

    const sseParser = createChatSSEParser({
      onReactEvent: (data) => { processReActEvent(data) },
      onStreamEvent: (data) => { processStreamEvent(data) }
    })

    const { runStreamFetch } = createSendStreamLifecycle({
      isGroupChat: false,
      assistantIdx,
      pendingAgentIds,
      assistantEntries,
      localTurns: localTurns as Ref<ChatSendStreamLocalTurnRow[]>,
      thoughtStatus,
      thoughtSteps,
      sessionId,
      currentGroup: ref(null) as Ref<null>,
      lastTurnDurationMs,
      parseStreamChunk: (chunk) => sseParser.feed(chunk),
      parseStreamFinish: () => sseParser.finish(),
      scrollChatToBottom,
      t,
      setStoredGroupSessionId: (_groupId: number, _sid: string) => {},
      setStreamAbortController: (ac) => { streamAbortController = ac }
    })

    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    const body = {
      message: text,
      session_id: sessionId.value,
      agent_id: agentId.value,
      image_urls: imageUrlsForHistory,
      file_urls: fileUrls
    }

    const ac = new AbortController()
    await runStreamFetch({ url, headers, body, ac })
  }

  function onComposerKeydown (e: KeyboardEvent) {
    onChatInputEnterToSend(e, send)
  }

  async function send () {
    if (!canSend.value) return
    const text = draft.value.trim()
    const imgs = [...pendingImages.value]
    const files = [...pendingFiles.value]
    const streamMessage = computeStreamMessageLabel(text, imgs, files, t)
    const userLabel = streamMessage
    draft.value = ''
    pendingImages.value = []
    pendingFiles.value = []
    const lenBeforeUser = localTurns.value.length
    localTurns.value.push({
      role: 'user',
      content: userLabel,
      displayedContent: userLabel,
      image_data_urls: imgs.map(i => i.dataUrl),
      image_urls: [],
      file_urls: [],
      createdAt: new Date().toISOString()
    })
    sending.value = true
    try {
      const { image_urls: uploadedImageUrls, file_urls: uploadedFileUrls } = await uploadPendingChatAttachments(
        imgs,
        files,
        uploadFile,
        {
          t,
          onPartialImageUpload: () => {
            $q.notify({ type: 'warning', message: t('chatPartialImageUploadFailed') })
          }
        }
      )
      const lastUser = localTurns.value[localTurns.value.length - 1]
      lastUser.image_urls = uploadedImageUrls
      lastUser.image_data_urls = undefined
      lastUser.file_urls = uploadedFileUrls
      await sendStream(streamMessage, uploadedFileUrls, uploadedImageUrls)
      await refreshSessionsList()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } }; message?: string }
      while (localTurns.value.length > lenBeforeUser) {
        localTurns.value.pop()
      }
      draft.value = text
      pendingImages.value = imgs
      pendingFiles.value = files
      const msg =
        err.response?.data?.message ??
        (e instanceof Error ? e.message : null) ??
        '请求失败'
      $q.notify({ type: 'negative', message: msg, timeout: 9000 })
    } finally {
      sending.value = false
    }
  }

  async function stopStream () {
    if (!sending.value) return
    stopping.value = true
    try {
      streamAbortController?.abort()
      const sid = sessionId.value
      if (sid) {
        const token = LocalStorage.getItem('access') as string | null
        await api.post('/chat/stop', { session_id: sid }, {
          headers: token ? { Authorization: `Bearer ${token}` } : {}
        })
      }
    } catch {
      // ignore stop errors
    } finally {
      finalizeRunningPlanTasksOnStop()
      thoughtStatus.value = 'completed'
      stopping.value = false
      sending.value = false
    }
  }

  watch(agentId, () => {
    localTurns.value = []
    lastTurnDurationMs.value = null
    if (agentId.value != null && agentId.value >= 1) {
      const ag = agents.value.find(a => a.id === agentId.value)
      if (ag?.public_id) {
        LocalStorage.set(LAST_AGENT_PUBLIC_ID_KEY, ag.public_id)
      }
      syncAgentIdToRoute()
      void loadSessions()
    }
  })

  watch(
    () => route.params.agentId,
    () => {
      if (normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)) return
      const rawStr = routePathSegment()
      if (agents.value.length === 0) return
      if (rawStr !== '' && isLikelySessionUUID(rawStr)) {
        const found = findAgentFromRouteParam(rawStr, agents.value)
        if (!found) return
        if (agentId.value !== found.id) {
          agentId.value = found.id
        }
        return
      }
      const found = findAgentFromRouteParam(rawStr, agents.value)
      if (!found) return
      if (agentId.value !== found.id) {
        agentId.value = found.id
      }
      if ((found.public_id || '').trim() !== '' && /^\d+$/.test(rawStr.trim())) {
        void router.replace({ name: 'chat', params: { agentId: found.public_id }, query: {} })
      }
    }
  )

  watch(
    () =>
      `${route.name === 'chat' ? 'chat' : ''}\x00${routePathSegment()}\x00${normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)}`,
    async () => {
      if (route.name !== 'chat') return
      const querySid = normalizeRouteQuery(route.query[ROUTE_SESSION_Q] as string | string[] | undefined)
      if (querySid !== '') {
        if (sessionId.value !== querySid) {
          const sess = await resolveSessionFromAPI(querySid)
          const aid = sess?.agent_id
          if (sess && aid != null && aid >= 1) agentId.value = aid
          sessionId.value = querySid
          localTurns.value = []
          await loadMessages()
        }
        return
      }
      const rawStr = routePathSegment()
      if (rawStr !== '' && isLikelySessionUUID(rawStr)) {
        if (findAgentFromRouteParam(rawStr, agents.value)) return
        if (sessionId.value !== rawStr) {
          await openSessionById(rawStr)
        }
      }
    }
  )

  function onDocumentVisibility () {
    if (document.visibilityState !== 'visible') return
    void loadSkillRiskLookup()
    if (agentId.value == null || agentId.value < 1) return
    void refreshSessionsList()
  }

  function onEscStopStream (e: KeyboardEvent) {
    if (e.key !== 'Escape') return
    if (!sending.value || stopping.value) return
    e.preventDefault()
    void stopStream()
  }

  onMounted(async () => {
    await loadAgents()
    void loadSessions()
    void loadSkillRiskLookup()
    document.addEventListener('visibilitychange', onDocumentVisibility)
    window.addEventListener('keydown', onEscStopStream)
  })

  onUnmounted(() => {
    document.removeEventListener('visibilitychange', onDocumentVisibility)
    window.removeEventListener('keydown', onEscStopStream)
  })

  watch(sending, v => {
    if (!v) scrollChatToBottom()
  })

  function promptRenameSession (s: ChatSession) {
    $q.dialog({
      title: t('renameSession'),
      message: t('renameSessionHint'),
      prompt: {
        model: sessionTitle(s),
        type: 'text',
        maxlength: 512,
        isValid: (val: string) => val.trim().length <= 512
      },
      cancel: true,
      persistent: true
    }).onOk((newTitle: string) => {
      void renameSession(s.session_id, newTitle)
    })
  }

  function confirmDeleteSession (s: ChatSession) {
    $q.dialog({
      title: t('confirmDelete'),
      message: t('deleteChatSessionConfirm'),
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' },
      persistent: true
    }).onOk(() => {
      void deleteSession(s.session_id)
    })
  }

  async function deleteSession (sid: string) {
    try {
      await api.delete(`/chat/sessions/${encodeURIComponent(sid)}`)
      await loadSessions()
      if (sessionId.value === sid) {
        sessionId.value = null
      }
      $q.notify({ type: 'positive', message: t('deleteSuccess') })
    } catch (e) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || t('deleteFailed') })
    }
  }

  async function renameSession (sid: string, title: string) {
    try {
      await api.put(`/chat/sessions/${encodeURIComponent(sid)}`, { title })
      await refreshSessionsList()
      const upd = sessionsList.value.find(s => s.session_id === sid)
      if (upd) {
        const idx = sessionsBrowseList.value.findIndex(s => s.session_id === sid)
        if (idx >= 0) sessionsBrowseList.value[idx] = { ...upd }
      }
      $q.notify({ type: 'positive', message: t('saveSuccess') })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('saveFailed') })
    }
  }

  function isAssistantTypingSlot (idx: number): boolean {
    if (!sending.value) return false
    const msgs = displayMessages.value
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    const text = ((m as { displayedContent?: string }).displayedContent || m.content || '').trim()
    if (text !== '') return false
    if (idx !== msgs.length - 1) return false
    if (idx === msgs.length - 1) {
      for (let i = thoughtSteps.value.length - 1; i >= 0; i--) {
        const s = thoughtSteps.value[i]
        if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
          const tasks = (s.data.tasks as unknown[] | undefined) ?? []
          if (tasks.length > 0) return false
        }
      }
    }
    return true
  }

  function getCurrentPlanTasks (): { index: number; task: string; status: 'pending' | 'running' | 'done' | 'error'; details?: { text: string; tone: 'error' | 'muted' }[] }[] {
    for (let i = thoughtSteps.value.length - 1; i >= 0; i--) {
      const s = thoughtSteps.value[i]
      if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
        return (s.data.tasks as { index: number; task: string; status: 'pending' | 'running' | 'done' | 'error'; details?: { text: string; tone: 'error' | 'muted' }[] }[]) || []
      }
    }
    return []
  }

  function getPlanExecuteTasksFromMessage (m: { reactSteps?: ChatReactStep[] }): PlanTaskRowWeb[] {
    const rs = m.reactSteps
    if (!rs?.length) return []
    for (let i = rs.length - 1; i >= 0; i--) {
      const s = rs[i]
      if (s.type === 'plan' && s.data?.kind === 'plan_execute') {
        const raw = (s.data.tasks as PlanTaskRowWeb[]) || []
        return raw.map((r) => ({ ...r }))
      }
    }
    return []
  }

  const GENERIC_STREAM_FAILURE_ZH = '抱歉，本次回复未能生成。请稍后重试。'

  function isGenericStreamFailureText (s: string): boolean {
    const t = String(s ?? '').replace(/\s+/g, '').trim()
    if (t === '') return true
    return t === GENERIC_STREAM_FAILURE_ZH.replace(/\s+/g, '')
  }

  function shouldRenderPlanExecuteForMessage (idx: number): boolean {
    const msgs = displayMessages.value as Array<{ role: string, reactSteps?: ChatReactStep[] }>
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    if (getPlanExecuteTasksFromMessage(m).length === 0) return false
    for (let i = idx + 1; i < msgs.length; i++) {
      const next = msgs[i]
      if (next.role === 'user') return true
      if (next.role === 'assistant' && getPlanExecuteTasksFromMessage(next).length > 0) return false
    }
    if (sending.value && getCurrentPlanTasks().length > 0) return false
    return true
  }

  function shouldHideAssistantPlanMessage (idx: number): boolean {
    const msgs = displayMessages.value as Array<{ role: string, content?: string, displayedContent?: string, reactSteps?: ChatReactStep[] }>
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    if (getPlanExecuteTasksFromMessage(m).length === 0) return false
    if (shouldRenderPlanExecuteForMessage(idx)) return false
    const text = String(m.displayedContent || m.content || '')
    return isGenericStreamFailureText(text)
  }

  function shouldHideAssistantMessageText (idx: number): boolean {
    const msgs = displayMessages.value as Array<{ role: string, content?: string, displayedContent?: string, reactSteps?: ChatReactStep[] }>
    const m = msgs[idx]
    if (!m || m.role !== 'assistant') return false
    const text = String(m.displayedContent || m.content || '')
    if (!isGenericStreamFailureText(text)) return false
    const hasHistoryPlan = getPlanExecuteTasksFromMessage(m).length > 0
    const hasStreamingPlan = sending.value && idx === msgs.length - 1 && getCurrentPlanTasks().length > 0
    return hasHistoryPlan || hasStreamingPlan
  }

  function userMessageTextToDisplay (m: Parameters<typeof userMessageTextToDisplayI18n>[0]): string {
    return userMessageTextToDisplayI18n(m, t)
  }

  function openImagePreview (url: string): void {
    const src = resolveChatImageUrl(url)
    if (!isSafeImagePreviewSrc(src)) {
      $q.notify({ type: 'warning', message: t('chatImagePreviewUnsafe') })
      return
    }
    $q.dialog({
      component: ChatImagePreviewDialog,
      componentProps: { src }
    })
  }

  function openFilePreview (url: string): void {
    window.open(url, '_blank')
  }

  return {
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
    selectSession,
    formatSessionTime,
    formatChatMessageTime,
    chatMessageTimeLabel,
    chatDateDividerText,
    messageDateDividerAt,
    sessionTitle,
    promptRenameSession,
    confirmDeleteSession,
    deleteSession,
    showSessionRail,
    sessionRailCollapsed,
    sessionGroupBlocks,
    sessionRailPreviewBlocks,
    showViewAllSessions,
    sessionFullDialogOpen,
    sessionListModeTab,
    createSessionFromSidebar,
    onSessionRailAddClick,
    sessionBrowseGroupBlocks,
    sessionsBrowseList,
    sessionsBrowseLoading,
    sessionsBrowseLoadingMore,
    sessionsBrowseHasMore,
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
    maxChatImages,
    imageInputRef,
    clearPendingImages,
    removePendingImageAt,
    onImageSelected,
    onComposerPaste,
    onComposerDragOver,
    onComposerDrop,
    chatImageInputAccept: CHAT_IMAGE_INPUT_ACCEPT,
    chatDocumentInputAccept: CHAT_DOCUMENT_INPUT_ACCEPT,
    canSend,
    canStartSession,
    createSession,
    send,
    stopStream,
    thoughtSidebarOpen,
    thoughtSteps,
    thoughtStatus,
    toggleThoughtSidebar,
    currentStreamModelName,
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
    uploadFile,
    getFileIcon,
    getFileName,
    resolveChatImageUrl,
    userMessageImageUrls,
    userMessageTextToDisplay,
    openImagePreview,
    openFilePreview
  }
}

export type UseChatPageReturn = ReturnType<typeof createChatPageState>

export function useChatPage (): UseChatPageReturn {
  return createChatPageState()
}
