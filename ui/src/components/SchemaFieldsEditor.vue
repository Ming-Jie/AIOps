<template>
  <div class="schema-fields-editor">
    <div v-if="fields.length === 0" class="text-caption text-grey q-mb-sm">
      {{ t('wfSchemaFieldsEmpty') }}
    </div>

    <div v-for="(field, idx) in fields" :key="idx" class="schema-field-edit-row q-mb-sm">
      <div class="row q-col-gutter-sm">
        <div class="col-6">
          <q-input
            :model-value="field.name"
            outlined dense
            :label="t('wfSchemaFieldName')"
            class="config-input"
            @update:model-value="patchField(idx, 'name', $event)"
          />
        </div>
        <div class="col-6">
          <q-select
            :model-value="field.type"
            :options="typeOptions"
            outlined dense emit-value map-options
            :label="t('wfSchemaFieldType')"
            class="config-input"
            @update:model-value="patchField(idx, 'type', $event)"
          />
        </div>
        <div class="col-12">
          <q-input
            :model-value="field.description"
            outlined dense
            :label="t('wfStartFieldDesc')"
            class="config-input"
            @update:model-value="patchField(idx, 'description', $event)"
          />
        </div>
        <div class="col-12 row items-center justify-between">
          <q-checkbox
            :model-value="field.required"
            dense
            :label="t('wfRequired')"
            @update:model-value="patchField(idx, 'required', $event)"
          />
          <q-btn flat dense color="negative" icon="delete" :label="t('delete')" @click="removeField(idx)" />
        </div>
      </div>
      <q-separator v-if="idx < fields.length - 1" class="q-mt-sm" />
    </div>

    <q-btn flat dense color="primary" icon="add" :label="t('wfSchemaAddField')" @click="addField" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  fieldsFromJsonSchema,
  jsonSchemaFieldTypeOptions,
  jsonSchemaFromFields,
  type JsonSchemaFieldEdit
} from 'src/lib/jsonSchemaForm'

const props = defineProps<{
  modelValue?: Record<string, unknown> | null
}>()

const emit = defineEmits<{
  'update:modelValue': [v: Record<string, unknown> | null]
}>()

const { t } = useI18n()

const typeOptions = jsonSchemaFieldTypeOptions
const fields = ref<JsonSchemaFieldEdit[]>([])

watch(
  () => props.modelValue,
  (schema) => {
    fields.value = fieldsFromJsonSchema(schema)
  },
  { immediate: true, deep: true }
)

function emitSchema () {
  const valid = fields.value.filter(f => f.name.trim())
  if (valid.length === 0) {
    emit('update:modelValue', null)
    return
  }
  emit('update:modelValue', jsonSchemaFromFields(valid))
}

function addField () {
  const idx = fields.value.length + 1
  fields.value = [
    ...fields.value,
    { name: `field_${idx}`, type: 'string', description: '', required: false }
  ]
  emitSchema()
}

function removeField (idx: number) {
  fields.value = fields.value.filter((_, i) => i !== idx)
  emitSchema()
}

function patchField (idx: number, key: keyof JsonSchemaFieldEdit, value: unknown) {
  fields.value = fields.value.map((f, i) => {
    if (i !== idx) return f
    if (key === 'required') return { ...f, required: value === true }
    if (key === 'type') return { ...f, type: String(value) as JsonSchemaFieldEdit['type'] }
    if (key === 'name') return { ...f, name: value == null ? '' : String(value) }
    return { ...f, description: value == null ? '' : String(value) }
  })
  emitSchema()
}
</script>

<style scoped>
.schema-field-edit-row {
  padding: 8px;
  border: 1px solid #eee;
  border-radius: 6px;
  background: #fafafa;
}
.config-input {
  font-size: 13px;
}
</style>
