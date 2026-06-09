<template>
  <div class="config-content">
    <div class="config-group">
      <div class="config-label">{{ t('wfNodeName') }}</div>
      <q-input v-model="nodeLabel" outlined dense :placeholder="t('wfNodeNamePh')" class="config-input" />
    </div>
    <div class="config-group">
      <div class="config-label">{{ t('wfStartUserPrompt') }}</div>
      <div class="text-caption text-grey-7 q-mb-xs">{{ t('wfStartUserPromptHint') }}</div>
      <q-input
        :model-value="userPrompt"
        outlined
        dense
        type="textarea"
        rows="4"
        :placeholder="t('wfStartUserPromptPh')"
        class="config-input"
        @update:model-value="onUserPrompt"
      />
    </div>

    <div class="config-group">
      <div class="config-label">{{ t('wfStartInputFields') }}</div>
      <div class="text-caption text-grey-7 q-mb-sm">{{ t('wfStartInputFieldsHint') }}</div>

      <div v-if="inputFields.length === 0" class="text-caption text-grey q-mb-sm">
        {{ t('wfStartInputFieldsEmpty') }}
      </div>

      <div v-for="(field, idx) in inputFields" :key="idx" class="input-field-row q-mb-sm">
        <div class="row q-col-gutter-sm">
          <div class="col-6">
            <q-input
              :model-value="field.name"
              outlined dense
              :label="t('wfStartFieldName')"
              :placeholder="t('wfStartFieldNamePh')"
              class="config-input"
              @update:model-value="patchField(idx, 'name', $event)"
            />
          </div>
          <div class="col-6">
            <q-select
              :model-value="field.type"
              :options="fieldTypeOptions"
              outlined dense emit-value map-options
              :label="t('wfStartFieldType')"
              class="config-input"
              @update:model-value="patchField(idx, 'type', $event)"
            />
          </div>
          <div class="col-12">
            <q-input
              :model-value="field.label"
              outlined dense
              :label="t('wfStartFieldLabel')"
              class="config-input"
              @update:model-value="patchField(idx, 'label', $event)"
            />
          </div>
          <div class="col-12">
            <q-input
              :model-value="field.description || ''"
              outlined dense
              :label="t('wfStartFieldDesc')"
              class="config-input"
              @update:model-value="patchField(idx, 'description', $event)"
            />
          </div>
          <div class="col-12 row items-center justify-between">
            <q-checkbox
              :model-value="field.required === true"
              dense
              :label="t('wfRequired')"
              @update:model-value="patchField(idx, 'required', $event)"
            />
            <q-btn flat dense color="negative" icon="delete" :label="t('delete')" @click="removeField(idx)" />
          </div>
        </div>
        <q-separator v-if="idx < inputFields.length - 1" class="q-mt-sm" />
      </div>

      <q-btn flat dense color="primary" icon="add" :label="t('wfStartAddField')" @click="addField" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  inputFieldsToJsonSchema,
  normalizeInputField,
  type InputFieldSpec,
  type InputFieldType
} from 'src/lib/inputFields'

const props = defineProps<{
  nodeLabel: string
  config: Record<string, unknown>
}>()

const emit = defineEmits<{
  'update:label': [v: string]
  'update:config': [v: Record<string, unknown>]
}>()

const { t } = useI18n()

const fieldTypeOptions = computed(() => [
  { label: t('wfStartFieldTypeText'), value: 'text' },
  { label: t('wfStartFieldTypeParagraph'), value: 'paragraph' },
  { label: t('wfStartFieldTypeNumber'), value: 'number' },
  { label: t('wfStartFieldTypeCheckbox'), value: 'checkbox' },
  { label: t('wfStartFieldTypeJson'), value: 'json' },
  { label: t('wfStartFieldTypeSelect'), value: 'select' }
])

const nodeLabel = computed({
  get: () => props.nodeLabel,
  set: (v) => emit('update:label', v)
})

const userPrompt = computed(() => {
  const v = props.config.user_prompt
  return v == null ? '' : String(v)
})

const inputFields = computed((): InputFieldSpec[] => {
  const raw = props.config.input_fields
  if (!Array.isArray(raw)) return []
  return raw.map((item) => {
    if (!item || typeof item !== 'object' || Array.isArray(item)) return null
    const r = item as Record<string, unknown>
    const name = typeof r.name === 'string' ? r.name : ''
    const typeRaw = typeof r.type === 'string' ? r.type : 'text'
    const type = (['text', 'paragraph', 'number', 'checkbox', 'json', 'select'].includes(typeRaw)
      ? typeRaw
      : 'text') as InputFieldType
    const spec: InputFieldSpec = {
      name,
      type,
      label: typeof r.label === 'string' && r.label.trim() ? r.label : name,
      required: r.required === true
    }
    if (typeof r.description === 'string') spec.description = r.description
    return spec
  }).filter((f): f is InputFieldSpec => f != null)
})

function emitConfigWithFields (fields: InputFieldSpec[]) {
  const valid = fields
    .map(f => normalizeInputField(f))
    .filter((f): f is InputFieldSpec => f != null)
  const inputSchema = valid.length > 0 ? inputFieldsToJsonSchema(valid) : {}
  emit('update:config', {
    ...props.config,
    input_fields: fields,
    input_schema: inputSchema
  })
}

function onUserPrompt (v: string | number | null) {
  const s = v == null ? '' : String(v)
  emit('update:config', { ...props.config, user_prompt: s })
}

function addField () {
  const fields = [...inputFields.value]
  const idx = fields.length + 1
  fields.push({
    name: `input_${idx}`,
    type: 'text',
    label: `Input ${idx}`,
    required: false
  })
  emitConfigWithFields(fields)
}

function removeField (idx: number) {
  const fields = inputFields.value.filter((_, i) => i !== idx)
  emitConfigWithFields(fields)
}

function patchField (idx: number, key: keyof InputFieldSpec, value: unknown) {
  const fields = inputFields.value.map((f, i) => {
    if (i !== idx) return { ...f }
    const next = { ...f, [key]: value } as InputFieldSpec
    if (key === 'name' && typeof value === 'string') {
      next.name = value
    }
    if (key === 'type') {
      next.type = String(value) as InputFieldType
    }
    return next
  })
  emitConfigWithFields(fields)
}
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
.input-field-row {
  padding: 8px;
  border: 1px solid #eee;
  border-radius: 6px;
  background: #fafafa;
}
</style>
