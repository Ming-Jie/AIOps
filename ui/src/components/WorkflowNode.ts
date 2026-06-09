import { computed } from 'vue'
import { getNodeHeaderColor, getNodeIcon as getNodeIconFromLib } from 'src/lib/upstreamOutputs'

export interface WorkflowNodeData {
  nodeType: string
  label?: string
  agentId?: number | null
  config?: Record<string, unknown>
  inputSchema?: Record<string, unknown>
  outputSchema?: Record<string, unknown>
  runStatus?: 'success' | 'error'
  runDurationMs?: number
  runError?: string
}

export function useWorkflowNode (data: WorkflowNodeData, _selected: boolean) {
  const headerColor = computed(() => getNodeHeaderColor(data.nodeType))

  const nodeIcon = computed(() => getNodeIconFromLib(data.nodeType))

  const config = computed(() => {
    const c = data.config
    if (c && typeof c === 'object') return c as Record<string, unknown>
    return {}
  })

  const hasPrompt = computed(() => {
    const val = config.value.prompt_template
    return typeof val === 'string' && val.length > 0
  })

  const promptPreview = computed(() => {
    const val = config.value.prompt_template as string | undefined
    if (!val) return ''
    return val.length > 40 ? val.substring(0, 40) + '...' : val
  })

  const hasCondition = computed(() => {
    const val = config.value.condition
    return typeof val === 'string' && val.length > 0
  })

  const conditionText = computed(() => {
    const val = config.value.condition as string
    return val.length > 30 ? val.substring(0, 30) + '...' : val
  })

  const hasToolName = computed(() => {
    const val = config.value.tool_name
    return typeof val === 'string' && val.length > 0
  })

  const toolPreview = computed(() => {
    const val = config.value.tool_name
    if (typeof val !== 'string' || val.length === 0) return ''
    if (data.nodeType !== 'mcp') return val
    const configID = config.value.mcp_config_id
    if (configID == null || configID === '' || configID === 0) return val
    return `MCP #${configID} · ${val}`
  })

  const hasInputSchema = computed(() => {
    const s = data.inputSchema
    return s && typeof s === 'object' && 'properties' in s
  })

  const inputSchemaFieldCount = computed(() => {
    const s = data.inputSchema as Record<string, unknown> | undefined
    if (!s) return 0
    const p = s.properties as Record<string, unknown> | undefined
    return p ? Object.keys(p).length : 0
  })

  const hasOutputSchema = computed(() => {
    const s = data.outputSchema
    return s && typeof s === 'object' && 'properties' in s
  })

  const outputSchemaFieldCount = computed(() => {
    const s = data.outputSchema as Record<string, unknown> | undefined
    if (!s) return 0
    const p = s.properties as Record<string, unknown> | undefined
    return p ? Object.keys(p).length : 0
  })

  return {
    headerColor,
    nodeIcon,
    config,
    hasPrompt,
    promptPreview,
    hasCondition,
    conditionText,
    hasToolName,
    toolPreview,
    hasInputSchema,
    inputSchemaFieldCount,
    hasOutputSchema,
    outputSchemaFieldCount
  }
}
