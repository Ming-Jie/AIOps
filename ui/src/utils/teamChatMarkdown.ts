import DOMPurify from 'dompurify'
import { marked } from 'marked'

marked.setOptions({
  gfm: true,
  breaks: true
})

/** 团队对话 Web 端：保留 data:image（单聊 MdRenderer 会剥掉，供 IM 用文件名引用） */
export function renderTeamChatMarkdown (text: string): string {
  const t = text?.trim()
  if (!t) return ''
  const html = marked.parse(t, { async: false }) as string
  return DOMPurify.sanitize(html, {
    USE_PROFILES: { html: true }
  })
}
