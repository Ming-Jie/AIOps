import { declaredOutputs } from './upstreamOutputs'
import { buildInputSchemaFromOutputNames } from './schemaCatalog'

export interface UpstreamInputImport {
  schema: Record<string, unknown>
  defaultValues: Record<string, string>
}

export function buildInputImportFromUpstreamNode (
  sourceNodeId: string,
  sourceNodeType: string,
  sourceConfig: Record<string, unknown> = {},
  outputSchema?: Record<string, unknown> | null
): UpstreamInputImport | null {
  const outputs = declaredOutputs(sourceNodeType, sourceConfig, outputSchema)
  if (outputs.length === 0) return null
  const names = outputs.map(o => o.name)
  return buildInputSchemaFromOutputNames(names, sourceNodeId)
}

export function hasInputSchemaProperties (inputSchema: unknown): boolean {
  if (!inputSchema || typeof inputSchema !== 'object' || Array.isArray(inputSchema)) return false
  const props = (inputSchema as Record<string, unknown>).properties
  return !!props && typeof props === 'object' && !Array.isArray(props) &&
    Object.keys(props as Record<string, unknown>).length > 0
}

/** Whether upstream auto-import should replace / fill input_schema on this node. */
export function shouldAutoImportInputSchema (
  nodeType: string,
  inputSchema: unknown,
  options: { force?: boolean } = {}
): boolean {
  if (!nodeType || nodeType === 'start' || nodeType === 'end') return false
  if (nodeType === 'tool' || nodeType === 'mcp') {
    return !hasInputSchemaProperties(inputSchema)
  }
  if (options.force) return true
  return !hasInputSchemaProperties(inputSchema)
}

export function applyUpstreamInputImport (
  nodeData: Record<string, unknown>,
  payload: UpstreamInputImport,
  overwriteValues = true
): Record<string, unknown> {
  const base = (nodeData.config as Record<string, unknown>) || {}
  const prevValues = (base.input_values as Record<string, string>) || {}
  return {
    ...nodeData,
    inputSchema: payload.schema,
    config: {
      ...base,
      input_values: overwriteValues ? payload.defaultValues : { ...payload.defaultValues, ...prevValues }
    }
  }
}
