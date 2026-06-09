import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

export type LarkRegisterAppStatus =
  | 'pending'
  | 'qr_ready'
  | 'polling'
  | 'completed'
  | 'denied'
  | 'expired'
  | 'failed'
  | 'cancelled'

export type LarkRegisterAppSession = {
  session_id?: string
  status: LarkRegisterAppStatus
  qr_url?: string | null
  app_id?: string | null
  app_secret?: string | null
  operator_open_id?: string | null
  channel_bound?: boolean
  error?: string | null
  message?: string | null
}

const TERMINAL: ReadonlySet<LarkRegisterAppStatus> = new Set([
  'completed',
  'denied',
  'expired',
  'failed',
  'cancelled'
])

export function isLarkRegisterAppTerminalStatus (status: LarkRegisterAppStatus): boolean {
  return TERMINAL.has(status)
}

export async function startLarkRegisterAppSession (
  agentId: number,
  body: { auto_bind?: boolean; app_name?: string } = {}
): Promise<string> {
  const { data } = await api.post<APIResponse<{ session_id: string }>>(
    `/agents/${agentId}/im/lark/register-app`,
    {
      auto_bind: body.auto_bind ?? true,
      app_name: body.app_name
    }
  )
  return data.data?.session_id ?? ''
}

export async function getLarkRegisterAppSession (
  agentId: number,
  sessionId: string
): Promise<LarkRegisterAppSession> {
  const { data } = await api.get<APIResponse<LarkRegisterAppSession>>(
    `/agents/${agentId}/im/lark/register-app/${encodeURIComponent(sessionId)}`
  )
  return (data.data ?? { status: 'failed' }) as LarkRegisterAppSession
}

export async function cancelLarkRegisterAppSession (
  agentId: number,
  sessionId: string
): Promise<void> {
  await api.delete(`/agents/${agentId}/im/lark/register-app/${encodeURIComponent(sessionId)}`)
}
