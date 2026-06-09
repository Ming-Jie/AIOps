<template>
  <div class="variable-expr-field" ref="containerRef">
    <div class="expr-input-wrap" @click="focusInput">
      <div ref="highlightRef" class="expr-highlight" v-html="highlightedText" />
      <textarea
        ref="textareaRef"
        :value="modelValue"
        @input="onInput"
        @keydown="onKeydown"
        @scroll="syncHighlightScroll"
        class="expr-textarea"
        :placeholder="placeholder"
        :rows="rows"
        :disabled="disabled"
        spellcheck="false"
      />
    </div>
    <div class="expr-toolbar" v-if="showToolbar">
      <slot name="actions">
        <q-btn
          v-if="showPicker"
          flat
          dense
          size="sm"
          icon="data_object"
          color="primary"
          @click="pickerOpen = !pickerOpen"
        >
          <q-tooltip>插入变量</q-tooltip>
        </q-btn>
      </slot>
    </div>
    <q-menu v-model="pickerOpen" anchor="top right" self="bottom right">
      <q-list dense style="min-width: 260px; max-height: 420px; overflow-y: auto;">
        <template v-for="group in pickerGroups" :key="group.id">
          <template v-if="group.id.startsWith('node:')">
            <q-expansion-item
              dense
              expand-icon="expand_more"
              :label="group.label"
              default-opened
            >
              <q-item
                v-for="variable in group.variables"
                :key="variable.expression"
                clickable
                v-close-popup
                @click="insertAtCursor(formatGroupVarRef(group.id, variable.expression))"
              >
                <q-item-section>
                  <q-item-label>{{ variable.name }}</q-item-label>
                  <q-item-label caption class="var-expr">{{ formatGroupVarRef(group.id, variable.expression) }}</q-item-label>
                </q-item-section>
              </q-item>
            </q-expansion-item>
          </template>
          <template v-else>
            <q-item-label header>{{ group.label }}</q-item-label>
            <q-item
              v-for="variable in group.variables"
              :key="variable.expression"
              clickable
              v-close-popup
              @click="insertAtCursor(formatFlatVarRef(variable.expression))"
            >
              <q-item-section>
                <q-item-label>{{ variable.name }}</q-item-label>
                <q-item-label caption class="var-expr">{{ formatFlatVarRef(variable.expression) }}</q-item-label>
              </q-item-section>
            </q-item>
          </template>
        </template>
      </q-list>
    </q-menu>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  collectVariableSuggestions,
  formatNodeVarRef,
  formatWorkflowInputRef,
  type VariableGroup
} from 'src/lib/upstreamOutputs'
import { findStartNodeInputFields } from 'src/lib/inputFields'
import type { LatestDebugMap } from 'src/lib/localVariableTree'

interface UpstreamNode {
  label: string
  value: string
  nodeType: string
  outputFields?: string[]
}

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  rows?: number
  disabled?: boolean
  showPicker?: boolean
  showToolbar?: boolean
  upstreamNodes?: UpstreamNode[]
  /** Live graph nodes for config-driven discovery */
  graphNodes?: Array<{ id: string; data?: Record<string, unknown> }>
  graphEdges?: Array<{ source?: string; target?: string }>
  selectedNodeId?: string | null
  latestDebug?: LatestDebugMap
}>(), {
  placeholder: '',
  rows: 3,
  showPicker: true,
  showToolbar: true,
  upstreamNodes: () => [],
  graphNodes: () => [],
  graphEdges: () => [],
  selectedNodeId: null,
  latestDebug: () => ({})
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
const highlightRef = ref<HTMLElement | null>(null)
const containerRef = ref<HTMLElement | null>(null)
const pickerOpen = ref(false)

function syncHighlightScroll () {
  const ta = textareaRef.value
  const hl = highlightRef.value
  if (!ta || !hl) return
  hl.scrollTop = ta.scrollTop
  hl.scrollLeft = ta.scrollLeft
}

const highlightedText = computed(() => {
  const text = props.modelValue
  const escaped = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  return escaped.replace(
    /\{\{(.+?)\}\}/g,
    '<span class="var-pill" data-var="$1">$&</span>'
  )
})

const pickerGroups = computed((): VariableGroup[] => {
  if (props.selectedNodeId && props.graphNodes.length > 0) {
    const workflowInputs = findStartNodeInputFields(props.graphNodes)
    return collectVariableSuggestions(
      props.selectedNodeId,
      props.graphNodes,
      props.graphEdges,
      {
        includeWorkflowInputs: workflowInputs,
        latestDebug: props.latestDebug
      }
    )
  }

  const groups: VariableGroup[] = [
    {
      id: 'sys',
      label: 'System',
      variables: [
        { expression: 'sys.query', name: 'sys.query', typeHint: 'string' },
        { expression: 'sys.workflow_id', name: 'sys.workflow_id', typeHint: 'string' },
        { expression: 'sys.workflow_run_id', name: 'sys.workflow_run_id', typeHint: 'string' }
      ]
    }
  ]

  for (const node of props.upstreamNodes ?? []) {
    const fields = node.outputFields ?? []
    if (fields.length === 0) continue
    groups.push({
      id: `node:${node.value}`,
      label: node.label,
      variables: fields.map(name => ({ expression: name, name }))
    })
  }
  return groups
})

function formatFlatVarRef (expression: string): string {
  if (expression.startsWith('sys.')) return `{{${expression}}}`
  return formatWorkflowInputRef(expression)
}

function formatGroupVarRef (groupId: string, expression: string): string {
  if (groupId === 'sys') return `{{${expression}}}`
  if (groupId === 'workflow-input') return formatWorkflowInputRef(expression)
  const nodeId = groupId.startsWith('node:') ? groupId.slice(5) : groupId
  return formatNodeVarRef(nodeId, expression)
}

function onInput (e: Event) {
  const target = e.target as HTMLTextAreaElement
  emit('update:modelValue', target.value)
}

function onKeydown (e: KeyboardEvent) {
  if (e.key === 'Tab') {
    e.preventDefault()
    const ta = textareaRef.value
    if (!ta) return
    const start = ta.selectionStart
    const end = ta.selectionEnd
    const val = ta.value
    const newVal = val.slice(0, start) + '  ' + val.slice(end)
    emit('update:modelValue', newVal)
    requestAnimationFrame(() => {
      ta.selectionStart = ta.selectionEnd = start + 2
    })
  }
}

function insertAtCursor (text: string) {
  const ta = textareaRef.value
  if (!ta) return
  const start = ta.selectionStart
  const end = ta.selectionEnd
  const val = props.modelValue
  const newVal = val.slice(0, start) + text + val.slice(end)
  emit('update:modelValue', newVal)
  requestAnimationFrame(() => {
    ta.focus()
    ta.selectionStart = ta.selectionEnd = start + text.length
    syncHighlightScroll()
  })
}

function focusInput () {
  textareaRef.value?.focus()
}
</script>

<style scoped>
.variable-expr-field {
  position: relative;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  overflow: hidden;
  transition: border-color 0.2s;
}

.variable-expr-field:focus-within {
  border-color: #1890ff;
  box-shadow: 0 0 0 2px rgba(24, 144, 255, 0.1);
}

.expr-input-wrap {
  position: relative;
}

.expr-highlight {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 8px 12px;
  font-size: 13px;
  font-family: inherit;
  line-height: 1.5;
  white-space: pre-wrap;
  word-wrap: break-word;
  word-break: break-word;
  pointer-events: none;
  color: #333;
  overflow: hidden;
  z-index: 0;
}

.expr-textarea {
  position: relative;
  display: block;
  width: 100%;
  padding: 8px 12px;
  font-size: 13px;
  font-family: inherit;
  line-height: 1.5;
  border: none;
  outline: none;
  resize: vertical;
  background: transparent;
  color: transparent;
  caret-color: #333;
  -webkit-text-fill-color: transparent;
  z-index: 1;
}

.expr-textarea::placeholder {
  color: #bfbfbf;
}

.expr-toolbar {
  display: flex;
  justify-content: flex-end;
  padding: 2px 4px;
  background: #fafafa;
  border-top: 1px solid #f0f0f0;
}

.var-expr {
  font-family: monospace;
  font-size: 11px;
  color: #4fc3f7;
}

:deep(.var-pill) {
  background: #e6f7ff;
  color: #1890ff !important;
  border: 1px solid #91d5ff;
  border-radius: 3px;
  padding: 0 2px;
  font-family: inherit;
  font-size: inherit;
  line-height: inherit;
}
</style>
