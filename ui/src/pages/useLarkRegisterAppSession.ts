import { onUnmounted, ref } from 'vue'
import {
  cancelLarkRegisterAppSession,
  getLarkRegisterAppSession,
  isLarkRegisterAppTerminalStatus,
  startLarkRegisterAppSession,
  type LarkRegisterAppSession,
  type LarkRegisterAppStatus
} from 'src/api/larkRegisterApp'

const POLL_INTERVAL_MS = 1500

export const LARK_REGISTER_STATUS_LABELS: Record<LarkRegisterAppStatus, string> = {
  pending: '正在准备二维码…',
  qr_ready: '请使用飞书 / Lark 扫描二维码并确认授权',
  polling: '已扫码，等待你在客户端确认…',
  completed: '应用创建完成',
  denied: '你已拒绝授权',
  expired: '二维码已过期，请重新发起',
  failed: '创建失败',
  cancelled: '会话已取消'
}

export function useLarkRegisterAppSession (agentId: () => number | null) {
  const sessionId = ref<string | null>(null)
  const session = ref<LarkRegisterAppSession | null>(null)
  const starting = ref(false)
  const error = ref<string | null>(null)
  let pollTimer: ReturnType<typeof setInterval> | null = null

  function stopPoll () {
    if (pollTimer != null) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  async function cancelSession (force = false) {
    stopPoll()
    const aid = agentId()
    const sid = sessionId.value
    if (!aid || !sid) {
      sessionId.value = null
      session.value = null
      return
    }
    const st = session.value?.status
    if (!force && st && isLarkRegisterAppTerminalStatus(st)) {
      sessionId.value = null
      session.value = null
      return
    }
    try {
      await cancelLarkRegisterAppSession(aid, sid)
    } catch {
      /* session may already be gone */
    } finally {
      sessionId.value = null
      session.value = null
    }
  }

  function startPoll (
    onCompleted: (s: LarkRegisterAppSession) => void,
    onFailed: (msg: string) => void
  ) {
    stopPoll()
    pollTimer = setInterval(() => {
      void (async () => {
        const aid = agentId()
        const sid = sessionId.value
        if (!aid || !sid) return
        try {
          const next = await getLarkRegisterAppSession(aid, sid)
          session.value = next
          if (next.status === 'completed') {
            stopPoll()
            onCompleted(next)
            return
          }
          if (isLarkRegisterAppTerminalStatus(next.status)) {
            stopPoll()
            onFailed(next.error ?? next.message ?? LARK_REGISTER_STATUS_LABELS[next.status])
          }
        } catch (e: unknown) {
          stopPoll()
          const msg = e instanceof Error ? e.message : '轮询失败，请稍后重试'
          error.value = msg
          onFailed(msg)
        }
      })()
    }, POLL_INTERVAL_MS)
  }

  async function startSession (
    onCompleted: (s: LarkRegisterAppSession) => void,
    onFailed: (msg: string) => void,
    appName?: string
  ) {
    const aid = agentId()
    if (!aid || aid < 1) {
      onFailed('请先保存智能体后再扫码绑定')
      return
    }
    starting.value = true
    error.value = null
    session.value = null
    sessionId.value = null
    stopPoll()
    try {
      const sid = await startLarkRegisterAppSession(aid, { auto_bind: true, app_name: appName })
      sessionId.value = sid
      const initial = await getLarkRegisterAppSession(aid, sid)
      session.value = initial
      if (initial.status === 'completed') {
        onCompleted(initial)
        return
      }
      if (
        initial.status !== 'pending' &&
        initial.status !== 'qr_ready' &&
        initial.status !== 'polling'
      ) {
        onFailed(initial.error ?? initial.message ?? LARK_REGISTER_STATUS_LABELS[initial.status])
        return
      }
      startPoll(onCompleted, onFailed)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '无法发起扫码创建，请稍后重试'
      error.value = msg
      onFailed(msg)
    } finally {
      starting.value = false
    }
  }

  onUnmounted(() => {
    stopPoll()
  })

  return {
    sessionId,
    session,
    starting,
    error,
    startSession,
    cancelSession,
    stopPoll
  }
}
