import DOMPurify from 'dompurify'
import { marked } from 'marked'
import { stripWebToolEchoes } from 'src/utils/chatAttachments'

marked.setOptions({
  gfm: true,
  breaks: true
})

const DATA_IMAGE_RE = /data:image\/[a-z+]+;base64,[A-Za-z0-9+/=]+/g

/** Render assistant Markdown to sanitized HTML for v-html (web only). */
export function renderChatMarkdown (text: string): string {
  const t = text?.trim()
  if (!t) return ''
  // data:image shown via reactSteps; strip duplicate URIs from markdown text only.
  const cleaned = stripWebToolEchoes(t).replace(DATA_IMAGE_RE, '')
  const html = marked.parse(cleaned, { async: false }) as string
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true }
  })
}
