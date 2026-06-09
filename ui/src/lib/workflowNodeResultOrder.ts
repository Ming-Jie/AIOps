import type { Edge } from '@vue-flow/core'
import type { ExecuteWorkflowResponse } from 'src/api/types'

/** Topological order of executed nodes using canvas edges (fallback when API order missing). */
export function inferNodeOrderFromGraph (
  nodeResults: Record<string, unknown> | undefined,
  edgeList: Edge[]
): string[] {
  const keys = nodeResults ? Object.keys(nodeResults) : []
  if (keys.length === 0) return []
  const idSet = new Set(keys)
  const inDeg = new Map<string, number>()
  const adj = new Map<string, string[]>()
  for (const k of keys) {
    inDeg.set(k, 0)
    adj.set(k, [])
  }
  for (const e of edgeList) {
    if (!idSet.has(e.source) || !idSet.has(e.target)) continue
    inDeg.set(e.target, (inDeg.get(e.target) ?? 0) + 1)
    const arr = adj.get(e.source)
    if (arr) arr.push(e.target)
  }
  const q: string[] = []
  for (const [id, d] of inDeg) {
    if (d === 0) q.push(id)
  }
  const out: string[] = []
  while (q.length) {
    const u = q.shift()!
    out.push(u)
    for (const v of adj.get(u) || []) {
      const nd = (inDeg.get(v) ?? 0) - 1
      inDeg.set(v, nd)
      if (nd === 0) q.push(v)
    }
  }
  return out.length ? out : keys
}

export function resolveNodeResultOrder (
  result: ExecuteWorkflowResponse | null | undefined,
  edgeList: Edge[]
): string[] {
  const nr = result?.node_results
  if (!nr || typeof nr !== 'object' || Array.isArray(nr)) return []
  const map = nr as Record<string, unknown>
  if (result?.node_result_order?.length) {
    return result.node_result_order.filter(id => id in map)
  }
  return inferNodeOrderFromGraph(map, edgeList)
}

export function orderedNodeResultEntries (
  result: ExecuteWorkflowResponse | null | undefined,
  edgeList: Edge[]
): Array<[string, Record<string, unknown>]> {
  const nr = result?.node_results
  if (!nr || typeof nr !== 'object' || Array.isArray(nr)) return []
  const map = nr as Record<string, Record<string, unknown>>
  const order = resolveNodeResultOrder(result, edgeList)
  return order
    .filter(id => map[id] && typeof map[id] === 'object')
    .map(id => [id, map[id]] as [string, Record<string, unknown>])
}

export function orderedNodeResultList (
  result: ExecuteWorkflowResponse | null | undefined,
  edgeList: Edge[]
): Record<string, unknown>[] {
  return orderedNodeResultEntries(result, edgeList).map(([, row]) => row)
}
