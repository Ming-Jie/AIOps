import { ref, type Ref } from 'vue'
import { cloneSnapshot } from 'src/lib/workflowDsl'

export function useWorkflowHistory (maxHistory = 50) {
  const history = ref<any[]>([])
  const historyIndex = ref(-1)
  const canUndo = ref(false)
  const canRedo = ref(false)

  function pushSnapshot (nodes: any[], edges: any[]) {
    const snapshot = cloneSnapshot(nodes, edges)

    history.value = history.value.slice(0, historyIndex.value + 1)
    history.value.push(snapshot)

    if (history.value.length > maxHistory) {
      history.value.shift()
    }

    historyIndex.value = history.value.length - 1
    canUndo.value = historyIndex.value > 0
    canRedo.value = false
  }

  function undo (nodes: Ref<any[]>, edges: Ref<any[]>): boolean {
    if (historyIndex.value <= 0) return false

    historyIndex.value--
    const snapshot = history.value[historyIndex.value]
    if (snapshot) {
      nodes.value = cloneSnapshot(snapshot.nodes, snapshot.edges).nodes
      edges.value = cloneSnapshot(snapshot.nodes, snapshot.edges).edges
      canUndo.value = historyIndex.value > 0
      canRedo.value = true
      return true
    }
    return false
  }

  function redo (nodes: Ref<any[]>, edges: Ref<any[]>): boolean {
    if (historyIndex.value >= history.value.length - 1) return false

    historyIndex.value++
    const snapshot = history.value[historyIndex.value]
    if (snapshot) {
      nodes.value = cloneSnapshot(snapshot.nodes, snapshot.edges).nodes
      edges.value = cloneSnapshot(snapshot.nodes, snapshot.edges).edges
      canUndo.value = true
      canRedo.value = historyIndex.value < history.value.length - 1
      return true
    }
    return false
  }

  function reset () {
    history.value = []
    historyIndex.value = -1
    canUndo.value = false
    canRedo.value = false
  }

  return {
    history,
    historyIndex,
    canUndo,
    canRedo,
    pushSnapshot,
    undo,
    redo,
    reset
  }
}
