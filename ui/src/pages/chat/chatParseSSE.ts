type SSEPayload = {
  content?: string
  text?: string
  type?: string
  event_type?: string
  data?: Record<string, unknown>
}

export type ChatSSEParserHandlers = {
  onReactEvent?: (data: Record<string, unknown>) => void
  onStreamEvent?: (data: Record<string, unknown>) => void
}

/** Mirrors backend parseSSEAssistantPayload rules for user-visible assistant text. */
function extractVisibleChunk (parsed: SSEPayload): string {
  const content = String(parsed.content ?? parsed.text ?? '')
  const trimmed = content.trim()
  const type = parsed.type ?? ''
  const eventType = parsed.event_type ?? ''

  if (type === 'final_answer') {
    return trimmed
  }
  if (type === 'thought') {
    return ''
  }
  if (type === 'info' || type === 'error') {
    return trimmed
  }
  if (eventType === 'info') {
    return trimmed
  }
  if (type !== '' || eventType !== '') {
    return ''
  }
  return content
}

/**
 * Incremental SSE parser: feed raw fetch chunks, process each `data:` line once,
 * return accumulated user-visible assistant text.
 */
export function createChatSSEParser (handlers: ChatSSEParserHandlers = {}) {
  let lineBuf = ''
  let tokenAcc = ''
  let finalAnswer = ''

  function processPayload (payload: string): void {
    try {
      const parsed = JSON.parse(payload) as SSEPayload
      if (parsed.type === 'react_event') {
        handlers.onReactEvent?.((parsed.data ?? parsed) as Record<string, unknown>)
        return
      }
      if (parsed.type === 'stream_event') {
        handlers.onStreamEvent?.((parsed.data ?? parsed) as Record<string, unknown>)
        return
      }
      if (parsed.event_type) {
        handlers.onStreamEvent?.(parsed as Record<string, unknown>)
      } else if (
        parsed.type &&
        parsed.type !== 'final_answer' &&
        !['info', 'error'].includes(parsed.type)
      ) {
        handlers.onReactEvent?.(parsed as Record<string, unknown>)
      }
      if (parsed.type === 'final_answer' && String(parsed.content ?? '').trim() !== '') {
        finalAnswer = String(parsed.content)
        return
      }
      const chunk = extractVisibleChunk(parsed)
      if (chunk) {
        tokenAcc += chunk
      }
    } catch {
      if (payload !== '[DONE]') {
        tokenAcc += payload
      }
    }
  }

  function feed (chunk: string): string {
    if (!chunk) return visibleText()
    lineBuf += chunk
    const lines = lineBuf.split('\n')
    lineBuf = lines.pop() ?? ''
    for (const line of lines) {
      if (!line.startsWith('data: ')) continue
      const payload = line.slice(6).trim()
      if (!payload || payload === '[DONE]') continue
      processPayload(payload)
    }
    return visibleText()
  }

  function finish (): string {
    const tail = lineBuf.trim()
    if (tail.startsWith('data: ')) {
      const payload = tail.slice(6).trim()
      if (payload && payload !== '[DONE]') {
        processPayload(payload)
      }
    }
    lineBuf = ''
    return visibleText()
  }

  function visibleText (): string {
    if (tokenAcc.trim()) return tokenAcc
    return finalAnswer
  }

  return { feed, finish }
}
