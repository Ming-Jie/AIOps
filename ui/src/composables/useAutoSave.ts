import { ref, watch, type Ref } from 'vue'
import type { Edge, Node } from '@vue-flow/core'

type SaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

interface AutoSaveOptions {
  delay?: number
  onSave: () => Promise<boolean>
  nodes: Ref<Node[]>
  edges: Ref<Edge[]>
  enabled?: Ref<boolean>
}

export function useAutoSave (options: AutoSaveOptions) {
  const { delay = 1500, onSave, nodes, edges, enabled } = options
  const saveStatus = ref<SaveStatus>('idle')
  const lastSaved = ref<number | null>(null)
  const errorMessage = ref('')
  let timer: ReturnType<typeof setTimeout> | null = null
  let isSaving = false

  const enabledRef = enabled || ref(true)

  async function flush () {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
    if (!enabledRef.value || isSaving) return
    isSaving = true
    saveStatus.value = 'saving'
    try {
      const ok = await onSave()
      saveStatus.value = ok ? 'saved' : 'error'
      if (ok) {
        lastSaved.value = Date.now()
        errorMessage.value = ''
      } else {
        errorMessage.value = '保存失败'
      }
    } catch {
      saveStatus.value = 'error'
      errorMessage.value = '保存出错'
    } finally {
      isSaving = false
      setTimeout(() => {
        if (saveStatus.value === 'saved') {
          saveStatus.value = 'idle'
        }
      }, 2000)
    }
  }

  function scheduleSave () {
    if (timer) clearTimeout(timer)
    if (!enabledRef.value) return
    saveStatus.value = 'dirty'
    timer = setTimeout(flush, delay)
  }

  watch(
    [nodes as any, edges as any],
    () => { scheduleSave() },
    { deep: true }
  )

  function setEnabled (v: boolean) {
    enabledRef.value = v
  }

  return {
    saveStatus,
    lastSaved,
    errorMessage,
    flush,
    scheduleSave,
    setEnabled
  }
}
