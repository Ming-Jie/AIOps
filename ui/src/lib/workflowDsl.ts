import type { Edge, Node } from '@vue-flow/core'

interface GraphSnapshot {
  nodes: Node[]
  edges: Edge[]
}

interface WorkflowDslMeta {
  ui?: {
    node_labels?: Record<string, string>
  }
}

interface WorkflowDsl {
  nodes: Array<{
    id: string
    type: string
    label: string
    position: { x: number; y: number }
    config: Record<string, unknown>
    agent_id?: number
    input_schema?: Record<string, unknown> | null
    output_schema?: Record<string, unknown> | null
  }>
  edges: Array<{
    id: string
    source: string
    target: string
    source_handle?: string
    target_handle?: string
    label?: string
    condition?: string
  }>
  meta?: WorkflowDslMeta
  entry_node?: string
}

export function dslFromGraph (nodes: Node[], edges: Edge[]): WorkflowDsl {
  const nodeLabels: Record<string, string> = {}

  const dslNodes = nodes.map(n => {
    const data = (n.data || {}) as Record<string, unknown>
    const nodeType = (data.nodeType as string) || ''
    const label = (data.label as string) || ''
    const config = (data.config as Record<string, unknown>) || {}

    if (label) {
      nodeLabels[n.id] = label
    }

    return {
      id: n.id,
      type: nodeType,
      label,
      position: n.position || { x: 0, y: 0 },
      config,
      agent_id: data.agentId as number | undefined,
      input_schema: data.inputSchema as Record<string, unknown> | null | undefined,
      output_schema: data.outputSchema as Record<string, unknown> | null | undefined
    }
  })

  const dslEdges = edges.map(e => ({
    id: e.id,
    source: e.source,
    target: e.target,
    source_handle: (e.sourceHandle as string) || undefined,
    target_handle: (e.targetHandle as string) || undefined,
    label: (e.label as string) || undefined,
    condition: ((e.data as Record<string, unknown>)?.condition as string) || undefined
  }))

  const startNode = nodes.find(n => {
    const data = (n.data || {}) as Record<string, unknown>
    return (data.nodeType as string) === 'start'
  })

  const dsl: WorkflowDsl = {
    nodes: dslNodes,
    edges: dslEdges
  }

  if (Object.keys(nodeLabels).length > 0) {
    dsl.meta = { ui: { node_labels: nodeLabels } }
  }

  if (startNode) {
    dsl.entry_node = startNode.id
  }

  return dsl
}

export function graphFromDsl (dsl: WorkflowDsl): GraphSnapshot {
  const nodes: Node[] = (dsl.nodes || []).map(n => ({
    id: n.id,
    type: 'default',
    position: n.position || { x: 100, y: 100 },
    data: {
      label: n.label || '',
      nodeType: n.type,
      agentId: n.agent_id || null,
      config: n.config || {},
      inputSchema: n.input_schema || null,
      outputSchema: n.output_schema || null
    }
  }))

  const edges: Edge[] = (dsl.edges || []).map(e => {
    const edge: Edge = {
      id: e.id,
      source: e.source,
      target: e.target
    }
    if (e.source_handle) edge.sourceHandle = e.source_handle
    if (e.target_handle) edge.targetHandle = e.target_handle
    if (e.label) edge.label = e.label
    if (e.condition) {
      edge.data = { condition: e.condition }
      edge.style = { stroke: '#FF9800' }
    }
    return edge
  })

  return { nodes, edges }
}

export function cloneSnapshot (nodes: Node[], edges: Edge[]): GraphSnapshot {
  return {
    nodes: JSON.parse(JSON.stringify(nodes)),
    edges: JSON.parse(JSON.stringify(edges))
  }
}

export function seedGraph (): GraphSnapshot {
  const startId = `node_0_${Date.now()}`
  const endId = `node_1_${Date.now() + 1}`
  return {
    nodes: [
      {
        id: startId,
        type: 'default',
        position: { x: 200, y: 200 },
        data: {
          label: 'Start',
          nodeType: 'start',
          agentId: null,
          config: { user_prompt: '', input_schema: {} },
          inputSchema: null,
          outputSchema: null
        }
      },
      {
        id: endId,
        type: 'default',
        position: { x: 500, y: 200 },
        data: {
          label: 'End',
          nodeType: 'end',
          agentId: null,
          config: { output_mapping: '' },
          inputSchema: null,
          outputSchema: null
        }
      }
    ],
    edges: [
      {
        id: `edge_${startId}_${endId}_${Date.now()}`,
        source: startId,
        target: endId
      }
    ]
  }
}
