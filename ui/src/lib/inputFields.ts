export type InputFieldType = 'text' | 'paragraph' | 'number' | 'checkbox' | 'json' | 'select'

export interface InputFieldSpec {
  name: string
  type: InputFieldType
  label: string
  description?: string
  required?: boolean
  default?: unknown
  options?: string[]
}

const VALID_TYPES = new Set<string>(['text', 'paragraph', 'number', 'checkbox', 'json', 'select'])
const NAME_RE = /^[a-zA-Z_][a-zA-Z0-9_]*$/

function jsonSchemaType (fieldType: InputFieldType): string {
  switch (fieldType) {
    case 'number': return 'number'
    case 'checkbox': return 'boolean'
    case 'json': return 'object'
    default: return 'string'
  }
}

export function normalizeInputField (raw: unknown): InputFieldSpec | null {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null
  const item = raw as Record<string, unknown>
  const name = typeof item.name === 'string' ? item.name.trim() : ''
  if (!name || !NAME_RE.test(name)) return null
  const typeRaw = typeof item.type === 'string' ? item.type : 'text'
  const type = VALID_TYPES.has(typeRaw) ? typeRaw as InputFieldType : 'text'
  const label = typeof item.label === 'string' && item.label.trim()
    ? item.label.trim()
    : name
  const description = typeof item.description === 'string' ? item.description : undefined
  const required = item.required === true
  const options = Array.isArray(item.options)
    ? item.options.filter((o): o is string => typeof o === 'string' && o.trim() !== '')
    : undefined
  const spec: InputFieldSpec = { name, type, label, required }
  if (description) spec.description = description
  if (item.default !== undefined) spec.default = item.default
  if (options && options.length > 0) spec.options = options
  return spec
}

export function parseInputFields (raw: unknown): InputFieldSpec[] {
  if (!Array.isArray(raw)) return []
  const out: InputFieldSpec[] = []
  for (const item of raw) {
    const spec = normalizeInputField(item)
    if (spec) out.push(spec)
  }
  return out
}

export function inputFieldsToJsonSchema (fields: InputFieldSpec[]): Record<string, unknown> {
  const properties: Record<string, unknown> = {}
  const required: string[] = []
  for (const f of fields) {
    const prop: Record<string, unknown> = {
      type: jsonSchemaType(f.type),
      title: f.label
    }
    if (f.description) prop.description = f.description
    if (f.type === 'select' && f.options?.length) {
      prop.enum = f.options
    }
    properties[f.name] = prop
    if (f.required) required.push(f.name)
  }
  const schema: Record<string, unknown> = { type: 'object', properties }
  if (required.length > 0) schema.required = required
  return schema
}

export function findStartNodeInputFields (nodes: Array<{ data?: Record<string, unknown> }>): InputFieldSpec[] {
  for (const node of nodes) {
    const data = node.data || {}
    if (data.nodeType !== 'start') continue
    const config = (data.config as Record<string, unknown>) || {}
    const fromFields = parseInputFields(config.input_fields)
    if (fromFields.length > 0) return fromFields
    const schema = config.input_schema
    if (schema && typeof schema === 'object' && !Array.isArray(schema)) {
      return jsonSchemaToInputFields(schema as Record<string, unknown>)
    }
    const nodeSchema = data.inputSchema
    if (nodeSchema && typeof nodeSchema === 'object' && !Array.isArray(nodeSchema)) {
      return jsonSchemaToInputFields(nodeSchema as Record<string, unknown>)
    }
  }
  return []
}

function jsonSchemaToInputFields (schema: Record<string, unknown>): InputFieldSpec[] {
  const props = schema.properties
  if (!props || typeof props !== 'object' || Array.isArray(props)) return []
  const required = new Set<string>()
  if (Array.isArray(schema.required)) {
    for (const r of schema.required) {
      if (typeof r === 'string') required.add(r)
    }
  }
  return Object.entries(props as Record<string, unknown>).map(([name, raw]) => {
    const field = raw && typeof raw === 'object' && !Array.isArray(raw)
      ? raw as Record<string, unknown>
      : {}
    const typeRaw = typeof field.type === 'string' ? field.type : 'string'
    let type: InputFieldType = 'text'
    if (typeRaw === 'number' || typeRaw === 'integer') type = 'number'
    else if (typeRaw === 'boolean') type = 'checkbox'
    else if (typeRaw === 'object') type = 'json'
    else if (Array.isArray(field.enum)) type = 'select'
    const label = typeof field.title === 'string' ? field.title : name
    const description = typeof field.description === 'string' ? field.description : undefined
    const spec: InputFieldSpec = { name, type, label, required: required.has(name) }
    if (description) spec.description = description
    if (Array.isArray(field.enum)) {
      spec.options = field.enum.filter((o): o is string => typeof o === 'string')
    }
    return spec
  }).filter(f => NAME_RE.test(f.name))
}
