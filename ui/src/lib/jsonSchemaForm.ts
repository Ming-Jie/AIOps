/**
 * Lightweight JSON Schema field editor helpers (object.properties only).
 */

export type JsonSchemaFieldType = 'string' | 'number' | 'integer' | 'boolean' | 'object' | 'array'

export interface JsonSchemaFieldEdit {
  name: string
  type: JsonSchemaFieldType
  description: string
  required: boolean
}

const FIELD_TYPES: JsonSchemaFieldType[] = ['string', 'number', 'integer', 'boolean', 'object', 'array']

function schemaRequiredSet (schema: Record<string, unknown>): Set<string> {
  const out = new Set<string>()
  const raw = schema.required
  if (Array.isArray(raw)) {
    raw.forEach(v => {
      if (typeof v === 'string' && v.trim()) out.add(v.trim())
    })
  }
  return out
}

function normalizeFieldType (raw: unknown): JsonSchemaFieldType {
  if (typeof raw === 'string' && (FIELD_TYPES as string[]).includes(raw)) {
    return raw as JsonSchemaFieldType
  }
  return 'string'
}

export function fieldsFromJsonSchema (schema: unknown): JsonSchemaFieldEdit[] {
  if (!schema || typeof schema !== 'object' || Array.isArray(schema)) return []
  const root = schema as Record<string, unknown>
  const props = root.properties
  if (!props || typeof props !== 'object' || Array.isArray(props)) return []
  const required = schemaRequiredSet(root)
  return Object.entries(props as Record<string, unknown>).map(([name, raw]) => {
    const field = raw && typeof raw === 'object' && !Array.isArray(raw)
      ? raw as Record<string, unknown>
      : {}
    const description = typeof field.description === 'string' ? field.description : ''
    const type = normalizeFieldType(field.type)
    return {
      name,
      type,
      description,
      required: field.required === true || required.has(name)
    }
  })
}

export function jsonSchemaFromFields (fields: JsonSchemaFieldEdit[]): Record<string, unknown> {
  const properties: Record<string, unknown> = {}
  const required: string[] = []
  for (const field of fields) {
    const name = field.name.trim()
    if (!name) continue
    const prop: Record<string, unknown> = { type: field.type }
    if (field.description.trim()) prop.description = field.description.trim()
    properties[name] = prop
    if (field.required) required.push(name)
  }
  const schema: Record<string, unknown> = { type: 'object', properties }
  if (required.length > 0) schema.required = required
  return schema
}

export const jsonSchemaFieldTypeOptions = FIELD_TYPES.map(value => ({ label: value, value }))
