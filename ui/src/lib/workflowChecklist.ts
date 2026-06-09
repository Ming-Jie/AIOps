import type { Edge, Node } from '@vue-flow/core'

export type ChecklistIssueLevel = 'error' | 'warning'

export interface ChecklistIssue {
  id: string
  level: ChecklistIssueLevel
  nodeId?: string
  message: string
}

type TranslateFn = (key: string, params?: Record<string, unknown>) => string

function resolveNodeAgentId (d: Record<string, unknown>, nodeType: string): number | undefined {
  const fromData = d.agentId
  if (fromData != null && fromData !== '') {
    const n = typeof fromData === 'number' ? fromData : Number(fromData)
    if (!Number.isNaN(n) && n >= 1) return n
  }
  if (nodeType === 'agent') {
    const cfg = (d.config as Record<string, unknown>) || {}
    for (const key of ['agent_id', 'agentId'] as const) {
      const raw = cfg[key]
      if (raw != null && raw !== '' && raw !== 0) {
        const n = typeof raw === 'number' ? raw : Number(raw)
        if (!Number.isNaN(n) && n >= 1) return n
      }
    }
  }
  return undefined
}

function schemaRequiredNames (schema: Record<string, unknown>): Set<string> {
  const out = new Set<string>()
  if (Array.isArray(schema.required)) {
    schema.required.forEach(v => {
      if (typeof v === 'string' && v.trim()) out.add(v.trim())
    })
  }
  const props = schema.properties
  if (props && typeof props === 'object' && !Array.isArray(props)) {
    for (const [name, raw] of Object.entries(props as Record<string, unknown>)) {
      const field = raw && typeof raw === 'object' && !Array.isArray(raw)
        ? raw as Record<string, unknown>
        : {}
      if (field.required === true) out.add(name)
    }
  }
  return out
}

export function buildWorkflowChecklist (
  nodes: Node[],
  edges: Edge[],
  t: TranslateFn
): ChecklistIssue[] {
  const issues: ChecklistIssue[] = []

  if (nodes.length === 0) {
    issues.push({ id: 'empty', level: 'error', message: t('wfChecklistEmpty') })
    return issues
  }

  const startNodes = nodes.filter(n => (n.data as Record<string, unknown>).nodeType === 'start')
  if (startNodes.length === 0) {
    issues.push({ id: 'no-start', level: 'error', message: t('wfChecklistNoStart') })
  } else if (startNodes.length > 1) {
    issues.push({ id: 'multi-start', level: 'warning', message: t('wfChecklistMultiStart') })
  }

  const endNodes = nodes.filter(n => (n.data as Record<string, unknown>).nodeType === 'end')
  if (endNodes.length === 0) {
    issues.push({ id: 'no-end', level: 'warning', message: t('wfChecklistNoEnd') })
  }

  const incoming = new Map<string, number>()
  for (const n of nodes) incoming.set(n.id, 0)
  for (const e of edges) {
    incoming.set(e.target, (incoming.get(e.target) ?? 0) + 1)
  }

  for (const n of nodes) {
    const d = n.data as Record<string, unknown>
    const nodeType = (d.nodeType as string) || ''
    const label = (d.label as string) || n.id

    if (nodeType !== 'start' && (incoming.get(n.id) ?? 0) === 0) {
      issues.push({
        id: `orphan-${n.id}`,
        level: 'warning',
        nodeId: n.id,
        message: t('wfChecklistOrphan', { label })
      })
    }

    if (nodeType === 'agent' && resolveNodeAgentId(d, nodeType) == null) {
      issues.push({
        id: `agent-${n.id}`,
        level: 'error',
        nodeId: n.id,
        message: t('wfAgentNodeNeedsAgent', { label })
      })
    }

    const inputSchema = d.inputSchema as Record<string, unknown> | undefined
    if (inputSchema && typeof inputSchema === 'object') {
      const required = schemaRequiredNames(inputSchema)
      if (required.size > 0) {
        const cfg = (d.config as Record<string, unknown>) || {}
        const inputValues = (cfg.input_values as Record<string, unknown>) || {}
        for (const name of required) {
          const val = inputValues[name]
          if (val == null || String(val).trim() === '') {
            issues.push({
              id: `input-${n.id}-${name}`,
              level: 'warning',
              nodeId: n.id,
              message: t('wfChecklistMissingInput', { label, field: name })
            })
          }
        }
      }
    }
  }

  return issues
}

export function checklistHasErrors (issues: ChecklistIssue[]): boolean {
  return issues.some(i => i.level === 'error')
}

export function checklistErrorCount (issues: ChecklistIssue[]): number {
  return issues.filter(i => i.level === 'error').length
}

export function checklistWarningCount (issues: ChecklistIssue[]): number {
  return issues.filter(i => i.level === 'warning').length
}
