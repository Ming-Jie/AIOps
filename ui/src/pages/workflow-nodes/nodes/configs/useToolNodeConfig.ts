import { onMounted, ref } from 'vue'
import { api } from 'boot/axios'
import type { APIResponse, Skill } from 'src/api/types'

export type ToolNameOption = { label: string; value: string }

function dedupeByValue (ordered: ToolNameOption[]): ToolNameOption[] {
  const seen = new Set<string>()
  return ordered.filter((o) => {
    if (seen.has(o.value)) return false
    seen.add(o.value)
    return true
  })
}

/** GET /skills：已启用且仅服务端执行（排除 execution_mode=client，与 workflow 在服务端跑 tool 一致） */
async function loadSkillToolOptions (): Promise<ToolNameOption[]> {
  try {
    const { data } = await api.get<APIResponse<Skill[]>>('/skills')
    const out: ToolNameOption[] = []
    for (const s of data.data ?? []) {
      if (!s.is_active) continue
      const mode = (s.execution_mode || '').trim().toLowerCase()
      if (mode !== 'server') continue
      const key = (s.key || '').trim()
      if (!key) continue
      const name = (s.name || '').trim()
      const label = name ? `${name} (${key})` : key
      out.push({ label, value: key })
    }
    return out
  } catch {
    return []
  }
}

/** Tool 节点只列服务端 Skill。MCP 调用使用专用 MCP 节点，避免服务选择和工具名混在一起。 */
export function useToolNodeNameOptions () {
  const loading = ref(false)
  const toolOptions = ref<ToolNameOption[]>([])

  async function load (): Promise<void> {
    loading.value = true
    try {
      toolOptions.value = dedupeByValue(await loadSkillToolOptions())
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void load()
  })

  return { loading, toolOptions, load }
}
