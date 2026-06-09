<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfMcpConfig') }}</div>
      <q-select
        :model-value="mcpConfigId"
        :options="mcpOptions"
        :loading="loadingMcp"
        outlined
        dense
        emit-value
        map-options
        clearable
        :placeholder="t('wfMcpConfigPh')"
        class="config-input"
        @update:model-value="onMcpConfigId"
      />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfMcpTool') }}</div>
      <q-select
        :model-value="strField('tool_name')"
        :options="toolOptions"
        :loading="loadingTools"
        outlined
        dense
        emit-value
        map-options
        clearable
        :disable="!mcpConfigId"
        :placeholder="t('wfMcpToolPh')"
        class="config-input"
        @update:model-value="onToolName"
      >
        <template #option="scope">
          <q-item v-bind="scope.itemProps">
            <q-item-section>
              <q-item-label>{{ scope.opt.label }}</q-item-label>
            </q-item-section>
          </q-item>
        </template>
        <template #selected-item="scope">
          {{ scope.opt?.label || t('wfMcpToolAuto') }}
        </template>
      </q-select>
      <div class="text-caption text-grey-6 q-mt-xs">{{ t('wfMcpToolAutoHint') }}</div>
    </div>

    <div v-if="hasCatalogSchema" class="config-group">
      <div class="text-caption text-positive">{{ t('wfCatalogSchemaApplied') }}</div>
    </div>

    <div class="config-group">
      <div class="config-label">{{ t('wfMcpArguments') }}</div>
      <WorkflowVariableField
        :model-value="strField('arguments')"
        :rows="6"
        :placeholder="t('wfMcpArgumentsPh')"
        @update:model-value="patchConfig('arguments', $event)"
      />
      <div class="text-caption text-grey-6 q-mt-xs">{{ t('wfMcpArgumentsHint') }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from 'boot/axios'
import type { APIResponse, MCPConfig, MCPTool } from 'src/api/types'
import WorkflowVariableField from 'components/WorkflowVariableField.vue'
import {
  argumentsJsonFromSchema,
  normalizeCatalogInputSchema
} from 'src/lib/schemaCatalog'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits<{
  'update:label': [v: string]
  'update:config': [v: Record<string, unknown>]
  'update:inputSchema': [v: Record<string, unknown> | null]
}>()

const { t } = useI18n()

const loadingMcp = ref(false)
const loadingTools = ref(false)
const mcpConfigs = ref<MCPConfig[]>([])
const mcpTools = ref<MCPTool[]>([])

const mcpOptions = computed(() =>
  mcpConfigs.value
    .filter(c => c.is_active)
    .map(c => ({ label: c.name ? `${c.name} (#${c.id})` : `#${c.id}`, value: c.id }))
)

const toolOptions = computed(() =>
  mcpTools.value
    .filter(t => t.is_active !== false && t.tool_name)
    .map(t => ({
      label: t.display_name?.trim() ? `${t.display_name} (${t.tool_name})` : t.tool_name,
      value: t.tool_name
    }))
)

const mcpConfigId = computed(() => {
  const v = props.config.mcp_config_id
  if (v == null || v === '' || v === 0) return null
  const n = typeof v === 'number' ? v : Number(v)
  return Number.isNaN(n) ? null : n
})

const hasCatalogSchema = computed(() => {
  const schema = props.config.tool_input_schema
  return !!schema && typeof schema === 'object'
})

const nodeLabel = computed({
  get: () => props.nodeLabel,
  set: (v) => emit('update:label', v)
})

function patchConfig (key: string, value: unknown) {
  emit('update:config', { ...props.config, [key]: value })
}

function strField (key: string): string {
  const v = props.config[key]
  return v == null ? '' : String(v)
}

function applyToolCatalog (toolName: string) {
  if (!toolName) {
    emit('update:inputSchema', null)
    emit('update:config', {
      ...props.config,
      tool_name: '',
      tool_input_schema: undefined
    })
    return
  }
  const tool = mcpTools.value.find(t => t.tool_name === toolName)
  const schema = normalizeCatalogInputSchema(tool?.input_schema)
  emit('update:inputSchema', schema)
  emit('update:config', {
    ...props.config,
    tool_name: toolName,
    tool_input_schema: tool?.input_schema ?? undefined,
    arguments: argumentsJsonFromSchema(schema)
  })
}

function onMcpConfigId (val: number | null) {
  emit('update:inputSchema', null)
  emit('update:config', {
    ...props.config,
    mcp_config_id: val == null ? 0 : val,
    tool_name: '',
    tool_input_schema: undefined,
    arguments: '{}'
  })
}

function onToolName (val: string | null) {
  applyToolCatalog(val ? String(val) : '')
}

async function loadMcpConfigs (): Promise<void> {
  loadingMcp.value = true
  try {
    const { data } = await api.get<APIResponse<MCPConfig[]>>('/mcp/configs')
    mcpConfigs.value = (data.data ?? []) as MCPConfig[]
  } catch {
    mcpConfigs.value = []
  } finally {
    loadingMcp.value = false
  }
}

async function loadMcpTools (configId: number | null): Promise<void> {
  if (!configId) {
    mcpTools.value = []
    return
  }
  loadingTools.value = true
  try {
    const { data } = await api.get<APIResponse<MCPTool[]>>(`/mcp/configs/${configId}/tools`)
    mcpTools.value = (data.data ?? []) as MCPTool[]
  } catch {
    mcpTools.value = []
  } finally {
    loadingTools.value = false
  }
}

watch(mcpConfigId, (id) => {
  void loadMcpTools(id)
}, { immediate: true })

onMounted(() => {
  void loadMcpConfigs()
})
</script>

<style scoped>
.config-content {
  padding: 12px 16px;
}
.config-group {
  margin-bottom: 16px;
}
.config-label {
  font-size: 13px;
  color: #333;
  margin-bottom: 6px;
  font-weight: 500;
}
.config-input {
  font-size: 13px;
}
</style>
