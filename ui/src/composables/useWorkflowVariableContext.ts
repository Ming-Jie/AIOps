import { inject, provide, type ComputedRef, type Ref } from 'vue'
import type { LatestDebugMap } from 'src/lib/localVariableTree'

export interface WorkflowVariableContext {
  selectedNodeId: Ref<string | null>
  nodes: Ref<Array<{ id: string; data?: Record<string, unknown> }>>
  edges: Ref<Array<{ source?: string; target?: string }>>
  latestDebug: Ref<LatestDebugMap> | ComputedRef<LatestDebugMap>
}

export const WORKFLOW_VAR_CONTEXT_KEY = Symbol('workflowVarContext')

export function provideWorkflowVariableContext (ctx: WorkflowVariableContext): void {
  provide(WORKFLOW_VAR_CONTEXT_KEY, ctx)
}

export function useWorkflowVariableContext (): WorkflowVariableContext | null {
  return inject(WORKFLOW_VAR_CONTEXT_KEY, null)
}
