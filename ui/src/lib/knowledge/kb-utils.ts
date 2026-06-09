/** Parse human-readable document name from OpenViking URI: viking://resources/kb/{id}/{docId}_{filename} */
export function parseDocNameFromUri (uri: string): string {
  if (!uri) return ''
  const slash = uri.lastIndexOf('/')
  const segment = slash >= 0 ? uri.slice(slash + 1) : uri
  const underscore = segment.indexOf('_')
  if (underscore > 0 && underscore < segment.length - 1) {
    return segment.slice(underscore + 1)
  }
  return segment || uri
}

export function formatKbCount (value: number | null | undefined): string {
  return Intl.NumberFormat('zh-CN').format(value ?? 0)
}

export type KbDocStatusKey = 'pending' | 'indexing' | 'indexed' | 'failed'

export function isKbDocIndexing (status: string): boolean {
  return status === 'indexing' || status === 'pending'
}
