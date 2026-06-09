import type { Edge, Node } from '@vue-flow/core'
import { declaredOutputs, type VariableSuggestion } from './upstreamOutputs'

export interface NextStepTarget {
  id: string
  label: string
  nodeType: string
}

export interface NextStep {
  output: VariableSuggestion
  targets: NextStepTarget[]
}

function toTarget (node: Node | undefined): NextStepTarget | null {
  if (!node) return null
  const data = (node.data || {}) as Record<string, unknown>
  const label = typeof data.label === 'string' && data.label.trim()
    ? data.label.trim()
    : node.id
  const nodeType = typeof data.nodeType === 'string' ? data.nodeType : ''
  return { id: node.id, label, nodeType }
}

function defaultOutput (): VariableSuggestion {
  return { expression: 'out', name: 'out', typeHint: 'any' }
}

export function buildNextSteps (
  nodeId: string,
  nodes: Node[],
  edges: Edge[],
  nodeType: string,
  config: Record<string, unknown> = {},
  outputSchema?: Record<string, unknown> | null
): NextStep[] {
  if (!nodeId) return []

  const nodeById = new Map(nodes.map(n => [n.id, n]))
  const outgoing = edges.filter(e => e.source === nodeId)
  const outputs = declaredOutputs(nodeType, config, outputSchema)
  const resolvedOutputs = outputs.length > 0 ? outputs : [defaultOutput()]

  const handleSet = new Set(
    outgoing
      .map(e => e.sourceHandle)
      .filter((h): h is string => typeof h === 'string' && h.length > 0)
  )
  const multiHandle = resolvedOutputs.length > 1 &&
    handleSet.size > 0 &&
    [...handleSet].some(h => resolvedOutputs.some(o => o.name === h))

  if (multiHandle) {
    return resolvedOutputs.map(output => ({
      output,
      targets: outgoing
        .filter(e => e.sourceHandle === output.name)
        .map(e => toTarget(nodeById.get(e.target)))
        .filter((t): t is NextStepTarget => t != null)
    }))
  }

  const targets = outgoing
    .map(e => toTarget(nodeById.get(e.target)))
    .filter((t): t is NextStepTarget => t != null)

  return [{
    output: resolvedOutputs[0] ?? defaultOutput(),
    targets
  }]
}

export function nextStepsHasConnections (steps: NextStep[]): boolean {
  return steps.some(s => s.targets.length > 0)
}
