import { parseInputFields, type InputFieldSpec } from './inputFields'
import { mergeWithDebugOutputs, type LatestDebugMap } from './localVariableTree'

export interface VariableSuggestion {
  expression: string
  name: string
  typeHint?: string
}

export interface VariableGroup {
  id: string
  label: string
  variables: VariableSuggestion[]
}

type ConfigDict = Record<string, unknown>
type OutputExtractor = (
  nodeType: string,
  config: ConfigDict,
  outputSchema?: Record<string, unknown> | null
) => VariableSuggestion[]

const SYSTEM_VARIABLES: readonly VariableSuggestion[] = [
  { expression: 'sys.query', name: 'sys.query', typeHint: 'string' },
  { expression: 'sys.workflow_id', name: 'sys.workflow_id', typeHint: 'string' },
  { expression: 'sys.workflow_run_id', name: 'sys.workflow_run_id', typeHint: 'string' }
]

const _outputExtractors = new Map<string, OutputExtractor>()

export function registerOutputExtractor (type: string, extractor: OutputExtractor): void {
  _outputExtractors.set(type, extractor)
}

function objectKeyList (value: unknown): string[] {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return []
  return Object.keys(value as ConfigDict).filter(k => k.length > 0 && /^[a-zA-Z_]\w*$/.test(k))
}

function nonEmptyString (value: unknown): string | null {
  return typeof value === 'string' && value.length > 0 ? value : null
}

function schemaPropertyKeys (outputSchema?: Record<string, unknown> | null): VariableSuggestion[] {
  if (!outputSchema?.properties || typeof outputSchema.properties !== 'object') return []
  const props = outputSchema.properties as Record<string, unknown>
  return Object.keys(props).map(name => ({ expression: name, name }))
}

function inputFieldToSuggestion (field: InputFieldSpec): VariableSuggestion {
  const typeHint = field.type === 'number'
    ? 'number'
    : field.type === 'checkbox'
      ? 'boolean'
      : field.type === 'json'
        ? 'object'
        : 'string'
  return { expression: field.name, name: field.name, typeHint }
}

function extractStart (_type: string, cfg: ConfigDict): VariableSuggestion[] {
  const out: VariableSuggestion[] = [
    { expression: 'message', name: 'message', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
  const fields = parseInputFields(cfg.input_fields)
  for (const f of fields) {
    out.push(inputFieldToSuggestion(f))
  }
  const schema = cfg.input_schema
  if (schema && typeof schema === 'object' && !Array.isArray(schema)) {
    const props = (schema as ConfigDict).properties
    if (props && typeof props === 'object' && !Array.isArray(props)) {
      for (const name of Object.keys(props as ConfigDict)) {
        if (/^[a-zA-Z_]\w*$/.test(name) && !out.some(v => v.name === name)) {
          out.push({ expression: name, name })
        }
      }
    }
  }
  return out
}

function extractEnd (): VariableSuggestion[] {
  return [
    { expression: 'output', name: 'output', typeHint: 'object' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractAgent (): VariableSuggestion[] {
  return [
    { expression: 'content', name: 'content', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractLlm (_type: string, cfg: ConfigDict): VariableSuggestion[] {
  const outVar = nonEmptyString(cfg.output_var) || 'content'
  return [
    { expression: outVar, name: outVar, typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractTool (_type: string, cfg: ConfigDict): VariableSuggestion[] {
  const out: VariableSuggestion[] = [
    { expression: 'type', name: 'type', typeHint: 'string' },
    { expression: 'tool', name: 'tool', typeHint: 'string' },
    { expression: 'tool_meta', name: 'tool_meta', typeHint: 'object' }
  ]
  const toolName = nonEmptyString(cfg.tool_name)
  if (toolName) out.push({ expression: 'tool_name', name: 'tool_name', typeHint: 'string' })
  return out
}

function extractMcp (_type: string, cfg: ConfigDict): VariableSuggestion[] {
  const toolName = nonEmptyString(cfg.tool_name) || nonEmptyString(cfg.toolName)
  if (!toolName) {
    return [
      { expression: 'tools', name: 'tools', typeHint: 'array' },
      { expression: 'tool_name', name: 'tool_name', typeHint: 'string' },
      { expression: 'mcp_config_id', name: 'mcp_config_id', typeHint: 'number' },
      { expression: 'type', name: 'type', typeHint: 'string' }
    ]
  }
  return [
    { expression: 'tool_name', name: 'tool_name', typeHint: 'string' },
    { expression: 'tool_meta', name: 'tool_meta', typeHint: 'object' },
    { expression: 'result', name: 'result', typeHint: 'object' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractHttp (): VariableSuggestion[] {
  return [
    { expression: 'body', name: 'body', typeHint: 'string' },
    { expression: 'status_code', name: 'status_code', typeHint: 'number' },
    { expression: 'url', name: 'url', typeHint: 'string' },
    { expression: 'method', name: 'method', typeHint: 'string' },
    { expression: 'error', name: 'error', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractCode (_type: string, cfg: ConfigDict): VariableSuggestion[] {
  const outputs = cfg.outputs
  const out: VariableSuggestion[] = [
    { expression: 'output', name: 'output', typeHint: 'string' },
    { expression: 'error', name: 'error', typeHint: 'string' }
  ]
  if (outputs && typeof outputs === 'object' && !Array.isArray(outputs)) {
    for (const name of objectKeyList(outputs)) {
      if (!out.some(v => v.name === name)) {
        out.unshift({ expression: name, name, typeHint: 'object' })
      }
    }
  }
  return out
}

function extractVariable (_type: string, cfg: ConfigDict): VariableSuggestion[] {
  const out: VariableSuggestion[] = [
    { expression: 'assigned', name: 'assigned', typeHint: 'object' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
  const assignments = cfg.assignments
  if (assignments && typeof assignments === 'object' && !Array.isArray(assignments)) {
    for (const name of objectKeyList(assignments)) {
      out.push({ expression: `assigned.${name}`, name: `assigned.${name}`, typeHint: 'object' })
    }
  }
  return out
}

function extractCondition (): VariableSuggestion[] {
  return [
    { expression: 'result', name: 'result', typeHint: 'boolean' },
    { expression: 'branch', name: 'branch', typeHint: 'string' },
    { expression: 'condition', name: 'condition', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractKnowledge (): VariableSuggestion[] {
  return [
    { expression: 'results', name: 'results', typeHint: 'array' },
    { expression: 'query', name: 'query', typeHint: 'string' },
    { expression: 'top_k', name: 'top_k', typeHint: 'number' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractTemplate (): VariableSuggestion[] {
  return [
    { expression: 'output', name: 'output', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function extractMerge (): VariableSuggestion[] {
  return [
    { expression: 'outputs', name: 'outputs', typeHint: 'array' },
    { expression: 'result', name: 'result', typeHint: 'object' },
    { expression: 'mode', name: 'mode', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ]
}

function mergeWithSchema (
  base: VariableSuggestion[],
  outputSchema?: Record<string, unknown> | null
): VariableSuggestion[] {
  const merged = [...base]
  for (const item of schemaPropertyKeys(outputSchema)) {
    if (!merged.some(v => v.name === item.name)) merged.push(item)
  }
  return merged
}

function registerBuiltins (): void {
  registerOutputExtractor('start', extractStart)
  registerOutputExtractor('end', () => extractEnd())
  registerOutputExtractor('agent', () => extractAgent())
  registerOutputExtractor('llm', extractLlm)
  registerOutputExtractor('tool', extractTool)
  registerOutputExtractor('mcp', extractMcp)
  registerOutputExtractor('http', () => extractHttp())
  registerOutputExtractor('code', extractCode)
  registerOutputExtractor('variable', extractVariable)
  registerOutputExtractor('condition', () => extractCondition())
  registerOutputExtractor('knowledge', () => extractKnowledge())
  registerOutputExtractor('template', () => extractTemplate())
  registerOutputExtractor('merge', () => extractMerge())
  registerOutputExtractor('notify', () => [
    { expression: 'sent', name: 'sent', typeHint: 'boolean' },
    { expression: 'message', name: 'message', typeHint: 'string' },
    { expression: 'channel', name: 'channel', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ])
  registerOutputExtractor('ssh', () => [
    { expression: 'output', name: 'output', typeHint: 'string' },
    { expression: 'error', name: 'error', typeHint: 'string' },
    { expression: 'host', name: 'host', typeHint: 'string' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ])
  registerOutputExtractor('apitest', () => [
    { expression: 'result', name: 'result', typeHint: 'object' },
    { expression: 'response', name: 'response', typeHint: 'object' },
    { expression: 'status_code', name: 'status_code', typeHint: 'number' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ])
  registerOutputExtractor('datamask', () => [
    { expression: 'masked', name: 'masked', typeHint: 'object' },
    { expression: 'count', name: 'count', typeHint: 'number' },
    { expression: 'type', name: 'type', typeHint: 'string' }
  ])
}

registerBuiltins()

export function declaredOutputs (
  nodeType: string,
  config: ConfigDict = {},
  outputSchema?: Record<string, unknown> | null
): VariableSuggestion[] {
  const extractor = _outputExtractors.get(nodeType)
  const base = extractor ? extractor(nodeType, config, outputSchema) : []
  return mergeWithSchema(base, outputSchema)
}

function collectAncestorIds (targetNodeId: string, edges: Array<{ source?: string; target?: string }>): Set<string> {
  const incoming = new Map<string, string[]>()
  for (const edge of edges) {
    if (!edge.source || !edge.target) continue
    const list = incoming.get(edge.target) ?? []
    list.push(edge.source)
    incoming.set(edge.target, list)
  }
  const seen = new Set<string>()
  const queue = [...(incoming.get(targetNodeId) ?? [])]
  while (queue.length > 0) {
    const id = queue.shift()!
    if (seen.has(id)) continue
    seen.add(id)
    for (const next of incoming.get(id) ?? []) {
      if (!seen.has(next)) queue.push(next)
    }
  }
  return seen
}

export function collectVariableSuggestions (
  nodeId: string,
  nodes: Array<{ id: string; data?: Record<string, unknown> }>,
  edges: Array<{ source?: string; target?: string }>,
  options: { includeWorkflowInputs?: InputFieldSpec[]; latestDebug?: LatestDebugMap } = {}
): VariableGroup[] {
  const groups: VariableGroup[] = [
    { id: 'sys', label: 'System', variables: [...SYSTEM_VARIABLES] }
  ]

  if (options.includeWorkflowInputs?.length) {
    groups.push({
      id: 'workflow-input',
      label: 'Workflow Input',
      variables: options.includeWorkflowInputs.map(inputFieldToSuggestion)
    })
  }

  const ancestors = collectAncestorIds(nodeId, edges)
  for (const node of nodes) {
    if (!ancestors.has(node.id)) continue
    const data = node.data || {}
    const nodeType = (data.nodeType as string) || ''
    const config = (data.config as ConfigDict) || {}
    const outputSchema = data.outputSchema as Record<string, unknown> | null | undefined
    let variables = declaredOutputs(nodeType, config, outputSchema)
    const debugOut = options.latestDebug?.[node.id]?.output
    variables = mergeWithDebugOutputs(variables, debugOut)
    if (variables.length === 0) continue
    groups.push({
      id: `node:${node.id}`,
      label: (data.label as string) || node.id,
      variables
    })
  }
  return groups
}

export function formatNodeVarRef (nodeId: string, fieldExpression: string): string {
  if (fieldExpression.startsWith('sys.')) return `{{${fieldExpression}}}`
  return `{{${nodeId}.${fieldExpression}}}`
}

export function formatWorkflowInputRef (fieldName: string): string {
  return `{{${fieldName}}}`
}

/** @deprecated use declaredOutputs — kept for existing call sites */
export function getNodeOutputFields (
  nodeType: string,
  config: Record<string, unknown> = {},
  outputSchema?: Record<string, unknown> | null
): string[] {
  return declaredOutputs(nodeType, config, outputSchema).map(v => v.name)
}

export function getNodeIcon (nodeType: string): string {
  const icons: Record<string, string> = {
    start: 'play_circle',
    end: 'stop_circle',
    agent: 'smart_toy',
    llm: 'psychology',
    condition: 'call_split',
    merge: 'merge_type',
    http: 'http',
    code: 'code',
    tool: 'build',
    mcp: 'hub',
    knowledge: 'library_books',
    notify: 'send',
    ssh: 'terminal',
    variable: 'data_object',
    template: 'text_snippet',
    apitest: 'api',
    datamask: 'security'
  }
  return icons[nodeType] || 'hub'
}

export function getNodeHeaderColor (nodeType: string): string {
  const colors: Record<string, string> = {
    start: '#4caf50',
    end: '#f44336',
    agent: '#722ED1',
    llm: '#EB2F96',
    condition: '#ff5722',
    merge: '#673ab7',
    http: '#00bcd4',
    code: '#607d8b',
    tool: '#ff9800',
    mcp: '#00a67d',
    knowledge: '#8bc34a',
    notify: '#ff9800',
    ssh: '#4caf50',
    variable: '#ffc107',
    template: '#795548',
    apitest: '#2196f3',
    datamask: '#9c27b0'
  }
  return colors[nodeType] || '#999'
}
