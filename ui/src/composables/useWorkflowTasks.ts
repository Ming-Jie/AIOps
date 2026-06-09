import { onMounted, ref } from 'vue'
import { api } from 'boot/axios'
import type { APIResponse } from 'src/api/types'

export interface WorkflowTaskField {
  key: string
  label: string
  type: string
  required: boolean
  default?: unknown
  description?: string
  options?: string[]
}

export interface WorkflowTaskDefinition {
  type: string
  name: string
  description: string
  icon: string
  color: string
  category: string
  config_schema: Record<string, WorkflowTaskField>
}

const tasksCache = ref<WorkflowTaskDefinition[]>([])
const loaded = ref(false)
const loading = ref(false)

export function useWorkflowTasks () {
  async function load (): Promise<WorkflowTaskDefinition[]> {
    if (loaded.value) return tasksCache.value
    loading.value = true
    try {
      const { data } = await api.get<APIResponse<WorkflowTaskDefinition[]>>('/workflows/graph/tasks')
      tasksCache.value = (data.data ?? []) as WorkflowTaskDefinition[]
      loaded.value = true
      return tasksCache.value
    } catch {
      tasksCache.value = []
      return []
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    void load()
  })

  return { tasks: tasksCache, loading, load }
}

export function getTaskDefinition (type: string, tasks: WorkflowTaskDefinition[]): WorkflowTaskDefinition | undefined {
  return tasks.find(t => t.type === type)
}
