import { computed } from 'vue'

export type KnowledgeNodeConfigProps = {
  nodeLabel: string
  config: Record<string, unknown>
}

export type KnowledgeNodeConfigEmit = {
  (e: 'update:label', v: string): void
  (e: 'update:config', v: Record<string, unknown>): void
}

export function useKnowledgeNodeForm (props: KnowledgeNodeConfigProps, emit: KnowledgeNodeConfigEmit) {
  const bindKbId = computed(() => {
    const v = props.config.knowledge_base_id
    if (v == null || v === '' || v === 0) return null
    const n = typeof v === 'number' ? v : Number(v)
    return Number.isNaN(n) ? null : n
  })

  function patchConfig (key: string, value: unknown) {
    emit('update:config', { ...props.config, [key]: value })
  }

  function onKbId (val: number | null) {
    patchConfig('knowledge_base_id', val == null ? 0 : val)
  }

  function onKbSelect (val: number | number[] | null) {
    onKbId(Array.isArray(val) ? (val[0] ?? null) : val)
  }

  function strField (key: string): string {
    const v = props.config[key]
    return v == null ? '' : String(v)
  }

  function numField (key: string): number {
    const v = props.config[key]
    if (v == null || v === '') return 0
    const n = typeof v === 'number' ? v : Number(v)
    return Number.isNaN(n) ? 0 : n
  }

  function patchNumber (key: string, raw: string | number | null | undefined) {
    if (raw === '' || raw === null || raw === undefined) {
      patchConfig(key, 0)
      return
    }
    const n = typeof raw === 'number' ? raw : Number(raw)
    patchConfig(key, Number.isNaN(n) ? 0 : n)
  }

  const nodeLabel = computed({
    get: () => props.nodeLabel,
    set: (v) => emit('update:label', v)
  })

  return {
    bindKbId,
    patchConfig,
    onKbId,
    onKbSelect,
    strField,
    numField,
    patchNumber,
    nodeLabel
  }
}
