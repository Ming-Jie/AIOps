/**
 * Helpers for catalog-driven node input schemas (MCP tools, skills, etc.)
 */

export function isObjectJsonSchema (schema: unknown): schema is Record<string, unknown> {
  return !!schema && typeof schema === 'object' && !Array.isArray(schema)
}

/** Normalize MCP/JSON Schema into workflow node input_schema shape. */
export function normalizeCatalogInputSchema (schema: unknown): Record<string, unknown> | null {
  if (!isObjectJsonSchema(schema)) return null

  const root = { ...schema }
  if (!root.type) root.type = 'object'

  const props = root.properties
  if (!props || typeof props !== 'object' || Array.isArray(props)) {
    root.properties = {}
  }

  return root
}

/** Build default argument object from schema properties (empty strings / defaults). */
export function defaultValuesFromSchema (schema: Record<string, unknown> | null): Record<string, unknown> {
  if (!schema?.properties || typeof schema.properties !== 'object') return {}
  const props = schema.properties as Record<string, unknown>
  const out: Record<string, unknown> = {}
  for (const [name, raw] of Object.entries(props)) {
    const field = raw && typeof raw === 'object' && !Array.isArray(raw)
      ? raw as Record<string, unknown>
      : {}
    if (field.default !== undefined) {
      out[name] = field.default
    } else if (field.type === 'boolean') {
      out[name] = false
    } else if (field.type === 'number' || field.type === 'integer') {
      out[name] = 0
    } else if (field.type === 'object') {
      out[name] = {}
    } else if (field.type === 'array') {
      out[name] = []
    } else {
      out[name] = ''
    }
  }
  return out
}

/** Default skill/tool node schema when catalog has no JSON Schema. */
export function defaultToolInputSchema (toolName: string): Record<string, unknown> {
  return {
    type: 'object',
    properties: {
      input: {
        type: 'string',
        title: 'Tool Input',
        description: toolName ? `Input for skill ${toolName}` : 'Tool input payload'
      }
    }
  }
}

export function syncInputValuesFromSchema (
  schema: Record<string, unknown> | null,
  prev: Record<string, string> = {}
): Record<string, string> {
  if (!schema?.properties || typeof schema.properties !== 'object') return {}
  const props = schema.properties as Record<string, unknown>
  const next: Record<string, string> = {}
  for (const name of Object.keys(props)) {
    next[name] = prev[name] ?? ''
  }
  return next
}

export function argumentsJsonFromSchema (schema: Record<string, unknown> | null): string {
  const values = defaultValuesFromSchema(schema)
  if (Object.keys(values).length === 0) return '{}'
  return JSON.stringify(values, null, 2)
}

/** Build downstream input_schema property names from upstream declared outputs. */
export function buildInputSchemaFromOutputNames (
  outputNames: string[],
  upstreamNodeId: string
): { schema: Record<string, unknown>; defaultValues: Record<string, string> } {
  const properties: Record<string, unknown> = {}
  const defaultValues: Record<string, string> = {}
  for (const name of outputNames) {
    properties[name] = {
      type: 'string',
      title: name,
      description: `Reference upstream ${upstreamNodeId}.${name}`
    }
    defaultValues[name] = `{{${upstreamNodeId}.${name}}}`
  }
  return {
    schema: { type: 'object', properties },
    defaultValues
  }
}
