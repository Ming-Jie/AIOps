import type { ChatReactStep, FileAttachment } from 'src/api/types'
import { resolveChatImageUrl } from 'src/pages/chat/chatMessageDisplay'

/**
 * Web chat display helpers.
 * - Images: data:image in reactSteps → inline preview (not in markdown text).
 * - Files: attachment.url → download button (reachable from the browser).
 * IM (Lark/DingTalk) uses a separate backend path; do not apply IM rules here.
 */

const IM_FILE_MARKER_RE = /\[\[(?:lark_file|dingtalk_file|im_file):[^\]]+\]\]/g
const IM_FILE_MARKER_LINE_RE = /^.*\[\[(?:lark_file|dingtalk_file|im_file):[^\]]+\]\].*$/gm
const TOOL_FILE_LINE_RE = /^Random file generated:\s*\S+\s*$/gim
const TOOL_GENERATED_FILES_LINE_RE = /^[^\n]*📎 Generated files:\s*[^\n]*$/gim
const TOOL_FILE_CONTENT_LINE_RE = /^[^\n]*文件内容如下[：:][^\n]*$/gim
/** Standalone base64 blobs pasted as file body (not data:image previews). */
const STANDALONE_BASE64_LINE_RE = /^[A-Za-z0-9+/]{40,}={0,2}\s*$/gm

/** Max assistant bubble characters shown in web chat (matches backend MaxWebBubbleRunes). */
export const MAX_WEB_BUBBLE_CHARS = 12_000

/** Matches backend skills.WebContentOmittedFallback — brief text when body was stripped but attachments exist. */
export const WEB_CONTENT_OMITTED_FALLBACK = '文件已生成，请使用下方下载按钮获取完整内容。'

const GENERIC_STREAM_FAILURE_ZH = '抱歉，本次回复未能生成。请稍后重试。'

const WEB_TRUNCATION_NOTICE = '\n\n…（内容过长，请使用下方附件下载完整文件）'

/** Cap streamed/persisted assistant text so multi-MB payloads cannot freeze the browser. */
export function capWebDisplayText (text: string): string {
  const t = text ?? ''
  if (t.length <= MAX_WEB_BUBBLE_CHARS) return t
  return t.slice(0, MAX_WEB_BUBBLE_CHARS) + WEB_TRUNCATION_NOTICE
}

const SCREENSHOT_SAVED_LINE_RE = /^Screenshot saved as screenshot_\d+\.png\.?\s*$/gim
const DATA_IMAGE_URI_RE = /data:image\/[a-z+]+;base64,[A-Za-z0-9+/=]+/g

/** Strip IM-only markers and tool echoes from web markdown; keep LLM prose and /api/... file URLs. */
export function stripWebToolEchoes (text: string): string {
  return text
    .replace(IM_FILE_MARKER_RE, '')
    .replace(IM_FILE_MARKER_LINE_RE, '')
    .replace(/File saved for IM delivery\.[^\n]*/gi, '')
    .replace(/Include this marker in your final answer:[^\n]*/gi, '')
    .replace(TOOL_FILE_LINE_RE, '')
    .replace(TOOL_GENERATED_FILES_LINE_RE, '')
    .replace(TOOL_FILE_CONTENT_LINE_RE, '')
    .replace(STANDALONE_BASE64_LINE_RE, '')
    .replace(SCREENSHOT_SAVED_LINE_RE, '')
    .replace(DATA_IMAGE_URI_RE, '')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/** Web assistant bubble text: keep LLM copy + URLs; images render via reactSteps. */
export function sanitizeWebAssistantBubbleText (text: string): string {
  return capWebDisplayText(stripWebToolEchoes(text ?? ''))
}

function isGenericStreamFailureText (s: string): boolean {
  const t = String(s ?? '').replace(/\s+/g, '').trim()
  return t === GENERIC_STREAM_FAILURE_ZH.replace(/\s+/g, '')
}

/** Last ReAct final_answer step content, sanitized for the main bubble. */
export function finalAnswerFromReactSteps (steps?: ChatReactStep[]): string {
  if (!steps?.length) return ''
  for (let i = steps.length - 1; i >= 0; i--) {
    const step = steps[i]
    if (step.type !== 'final') continue
    const raw = step.data?.content
    if (typeof raw !== 'string') continue
    const sanitized = sanitizeWebAssistantBubbleText(raw)
    if (sanitized) return sanitized
  }
  return ''
}

/**
 * Mirrors backend pickPersistedWebAssistantText for live SSE accumulation.
 * Prefer final_answer when token acc is tool echo / inline payload stripped to empty or fallback.
 */
export function pickWebAssistantDisplayText (
  rawAcc: string,
  rawFinal: string,
  sanitizedAcc: string,
  sanitizedFinal: string
): string {
  const trimAcc = rawAcc.trim()
  const trimFinal = rawFinal.trim()
  if (sanitizedFinal && !isGenericStreamFailureText(sanitizedFinal)) {
    if (
      !sanitizedAcc ||
      isGenericStreamFailureText(sanitizedAcc) ||
      (sanitizedAcc === WEB_CONTENT_OMITTED_FALLBACK && trimFinal !== '' && trimAcc !== trimFinal)
    ) {
      return sanitizedFinal
    }
  }
  if (sanitizedAcc && !isGenericStreamFailureText(sanitizedAcc)) {
    return sanitizedAcc
  }
  return sanitizedFinal
}

/** Resolve assistant prose for the main bubble (attachments render separately below). */
export function resolveWebAssistantBubbleText (input: {
  content?: string
  reactSteps?: ChatReactStep[]
}): string {
  const raw = input.content ?? ''
  let sanitized = sanitizeWebAssistantBubbleText(raw)
  if (isGenericStreamFailureText(sanitized)) {
    sanitized = ''
  }
  if (sanitized) return sanitized

  const fromFinal = finalAnswerFromReactSteps(input.reactSteps)
  if (fromFinal) return fromFinal

  return ''
}

function attachmentKey (att: FileAttachment): string {
  return (att.url || att.inline || att.filename || '').trim()
}

export function dedupeAttachments (items: FileAttachment[]): FileAttachment[] {
  const seen = new Set<string>()
  const out: FileAttachment[] = []
  for (const att of items) {
    const key = attachmentKey(att)
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push(att)
  }
  return out
}

export function collectAttachmentsFromReactSteps (steps?: ChatReactStep[]): FileAttachment[] {
  if (!steps?.length) return []
  const all: FileAttachment[] = []
  for (const step of steps) {
    if (step.type !== 'observation') continue
    const raw = step.data?.attachments
    if (!Array.isArray(raw)) continue
    for (const item of raw) {
      if (!item || typeof item !== 'object') continue
      const att = item as FileAttachment
      if (!att.filename) continue
      all.push(att)
    }
  }
  return dedupeAttachments(all)
}

export function isImageFileAttachment (att: FileAttachment): boolean {
  const mime = (att.mime_type || '').toLowerCase()
  if (mime.startsWith('image/')) return true
  const name = (att.filename || '').toLowerCase()
  return /\.(png|jpe?g|gif|webp|bmp)$/i.test(name)
}

function extractDataImagePrefix (text: string): string {
  const line = text.split('\n')[0]?.trim() ?? ''
  return line.startsWith('data:image/') ? line : ''
}

/** Image src for thought-sidebar TOOL_RESULT (data URI or image attachment URL only). */
export function observationImageSrc (data?: Record<string, unknown>): string {
  if (!data) return ''
  const inline = data.inline_image
  if (typeof inline === 'string' && inline.startsWith('data:image/')) {
    return extractDataImagePrefix(inline) || inline
  }
  const content = data.content
  if (typeof content === 'string') {
    const src = extractDataImagePrefix(content)
    if (src) return src
  }
  const rawAtt = data.attachments
  if (!Array.isArray(rawAtt)) return ''
  for (const item of rawAtt) {
    if (!item || typeof item !== 'object') continue
    const att = item as FileAttachment
    if (!isImageFileAttachment(att)) continue
    const href = attachmentDownloadHref(att)
    if (href) return href
  }
  return ''
}

/** Text body for thought-sidebar TOOL_RESULT (strips embedded data:image and tool echoes). */
export function observationTextBody (data?: Record<string, unknown>, maxChars = 8000): string {
  if (!data) return ''
  let body = ''
  const c = data.content
  if (typeof c === 'string') body = c
  else if (c != null) body = String(c)
  if (body.startsWith('data:image/')) {
    const nl = body.indexOf('\n')
    body = nl >= 0 ? body.slice(nl + 1) : ''
  }
  const cleaned = capWebDisplayText(stripWebToolEchoes(body.trim()))
  if (!cleaned) return ''
  return cleaned.slice(0, maxChars + 120)
}

/** Non-image attachments for thought-sidebar when inline image already shown. */
export function observationFileAttachments (data?: Record<string, unknown>): FileAttachment[] {
  if (!data || !Array.isArray(data.attachments)) return []
  const hasInline = observationImageSrc(data) !== ''
  const out: FileAttachment[] = []
  for (const item of data.attachments) {
    if (!item || typeof item !== 'object') continue
    const att = item as FileAttachment
    if (!att.filename) continue
    if (hasInline && isImageFileAttachment(att)) continue
    out.push(att)
  }
  return out
}

export function collectInlineImagesFromReactSteps (steps?: ChatReactStep[]): string[] {
  if (!steps?.length) return []
  const seen = new Set<string>()
  const out: string[] = []
  for (const step of steps) {
    if (step.type !== 'observation') continue
    const candidates = [step.data?.inline_image, step.data?.content]
    for (const raw of candidates) {
      if (typeof raw !== 'string') continue
      const src = raw.startsWith('data:image/') ? extractDataImagePrefix(raw) || raw : ''
      if (!src || seen.has(src)) continue
      seen.add(src)
      out.push(src)
    }
  }
  return out
}

/** Inline data: URIs plus image/png attachment URLs when no inline preview on the same step. */
export function collectAssistantImagesFromReactSteps (steps?: ChatReactStep[]): string[] {
  if (!steps?.length) return []
  const seen = new Set<string>()
  const out: string[] = []
  const push = (src: string): void => {
    const t = src.trim()
    if (!t || seen.has(t)) return
    seen.add(t)
    out.push(t)
  }

  for (const step of steps) {
    if (step.type !== 'observation') continue

    let hasInline = false
    for (const raw of [step.data?.inline_image, step.data?.content]) {
      if (typeof raw !== 'string') continue
      const src = raw.startsWith('data:image/') ? extractDataImagePrefix(raw) || raw : ''
      if (!src) continue
      push(src)
      hasInline = true
    }

    if (hasInline) continue

    const rawAtt = step.data?.attachments
    if (!Array.isArray(rawAtt)) continue
    for (const item of rawAtt) {
      if (!item || typeof item !== 'object') continue
      const att = item as FileAttachment
      if (!att.filename || !isImageFileAttachment(att)) continue
      push(attachmentDownloadHref(att))
    }
  }
  return out
}

/** Non-image attachments only (image/png screenshots render above as previews). */
export function collectAssistantFileAttachments (steps?: ChatReactStep[]): FileAttachment[] {
  return collectAttachmentsFromReactSteps(steps).filter((att) => !isImageFileAttachment(att))
}

export function attachmentDownloadHref (att: FileAttachment): string {
  const href = (att.inline || att.url || '').trim()
  if (!href) return ''
  return resolveChatImageUrl(href)
}

export function formatAttachmentSize (bytes?: number): string {
  if (bytes == null || bytes <= 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}
