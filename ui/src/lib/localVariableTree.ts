import type { VariableSuggestion } from './upstreamOutputs'

const IDENT_RE = /^[A-Za-z_][A-Za-z0-9_]*$/
const MAX_DEPTH = 5
const MAX_CHILDREN = 100

function jsonTypeHint (value: unknown): string {
  if (value === null || value === undefined) return 'any'
  if (typeof value === 'boolean') return 'boolean'
  if (typeof value === 'number') return 'number'
  if (typeof value === 'string') return 'string'
  if (Array.isArray(value)) return 'array'
  if (typeof value === 'object') return 'object'
  return 'any'
}

function segmentExpr (parentExpr: string, key: string): string {
  if (IDENT_RE.test(key)) {
    return parentExpr ? `${parentExpr}.${key}` : key
  }
  const escaped = key.replace(/\\/g, '\\\\').replace(/"/g, '\\"')
  return parentExpr ? `${parentExpr}["${escaped}"]` : `["${escaped}"]`
}

interface ExpandBudget {
  left: number
}

function expandValue (
  parentExpr: string,
  value: unknown,
  depth: number,
  budget: ExpandBudget
): VariableSuggestion[] {
  if (depth >= MAX_DEPTH || budget.left <= 0) return []

  if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
    const out: VariableSuggestion[] = []
    const entries = value as Record<string, unknown>
    for (const key of Object.keys(entries).sort()) {
      if (budget.left <= 0) break
      const childValue = entries[key]
      const expr = segmentExpr(parentExpr, key)
      out.push({ expression: expr, name: expr, typeHint: jsonTypeHint(childValue) })
      budget.left--
      if (childValue !== null && typeof childValue === 'object') {
        out.push(...expandValue(expr, childValue, depth + 1, budget))
      }
    }
    return out
  }

  if (Array.isArray(value) && value.length > 0) {
    const expr = `${parentExpr}[0]`
    return expandValue(expr, value[0], depth + 1, budget)
  }

  return []
}

export function expandDebugOutputVariables (output: unknown): VariableSuggestion[] {
  const budget: ExpandBudget = { left: MAX_CHILDREN }
  return expandValue('', output, 0, budget)
}

export type LatestDebugMap = Record<string, { output?: unknown }>

export function mergeWithDebugOutputs (
  declared: VariableSuggestion[],
  debugOutput: unknown | undefined
): VariableSuggestion[] {
  if (debugOutput == null) return declared
  const merged = [...declared]
  const seen = new Set(merged.map(v => v.expression))
  for (const item of expandDebugOutputVariables(debugOutput)) {
    if (seen.has(item.expression)) continue
    seen.add(item.expression)
    merged.push(item)
  }
  return merged
}
