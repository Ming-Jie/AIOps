import type { WorkflowNodeCategory, WorkflowNodeType } from './types'

const nodeRegistry = new Map<string, WorkflowNodeType>()

export function registerNodeType (node: WorkflowNodeType) {
  nodeRegistry.set(node.value, node)
}

export function registerNodeTypes (nodes: WorkflowNodeType[]) {
  for (const node of nodes) {
    nodeRegistry.set(node.value, node)
  }
}

export function getNodeType (value: string): WorkflowNodeType | undefined {
  return nodeRegistry.get(value)
}

export function getAllNodeTypes (): WorkflowNodeType[] {
  return Array.from(nodeRegistry.values())
}

export function getNodeTypesByCategory (category: string): WorkflowNodeType[] {
  return getAllNodeTypes().filter(n => n.category === category)
}

export function getCategories (): WorkflowNodeCategory[] {
  const categoryMap = new Map<string, WorkflowNodeCategory>()

  for (const node of getAllNodeTypes()) {
    if (!categoryMap.has(node.category)) {
      categoryMap.set(node.category, {
        name: node.category,
        icon: getCategoryIcon(node.category),
        nodes: []
      })
    }
    const cat = categoryMap.get(node.category)
    if (cat) {
      cat.nodes.push(node)
    }
  }

  return Array.from(categoryMap.values())
}

function getCategoryIcon (category: string): string {
  const icons: Record<string, string> = {
    flow: 'account_tree',
    ai: 'smart_toy',
    action: 'build',
    data: 'data_usage',
    ops: 'monitor',
    notify: 'notifications',
    test: 'science'
  }
  return icons[category] || 'widgets'
}

export type { WorkflowNodeType, WorkflowNodeCategory }
