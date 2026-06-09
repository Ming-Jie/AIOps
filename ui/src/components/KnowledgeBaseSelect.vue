<template>
  <q-select
    :model-value="modelValue"
    :options="filteredOptions"
    :loading="loading"
    outlined
    dense
    emit-value
    map-options
    clearable
    use-input
    input-debounce="200"
    :multiple="multiple"
    :use-chips="multiple"
    :placeholder="placeholder || t('wfSelectKbPh')"
    class="kb-select"
    @filter="onFilter"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <template #no-option>
      <q-item>
        <q-item-section class="text-grey">{{ t('kbSelectEmpty') }}</q-item-section>
      </q-item>
    </template>
    <template #option="scope">
      <q-item v-bind="scope.itemProps">
        <q-item-section>
          <q-item-label>{{ scope.opt.label }}</q-item-label>
          <q-item-label caption>
            #{{ scope.opt.value }}
            <span v-if="scope.opt.docCount != null"> · {{ t('kbDocCount', { n: scope.opt.docCount }) }}</span>
          </q-item-label>
        </q-item-section>
        <q-item-section v-if="scope.opt.visibility" side>
          <q-badge
            :color="scope.opt.visibility === 'public' ? 'blue' : 'grey'"
            :label="scope.opt.visibility === 'public' ? t('kbPublic') : t('kbPrivate')"
          />
        </q-item-section>
      </q-item>
    </template>
  </q-select>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from 'boot/axios'
import type { APIResponse, KnowledgeBase } from 'src/api/types'

type KbOption = {
  label: string
  value: number
  docCount: number
  visibility: string
}

defineProps<{
  modelValue: number | number[] | null
  multiple?: boolean
  placeholder?: string
}>()

defineEmits<{
  'update:modelValue': [v: number | number[] | null]
}>()

const { t } = useI18n()
const loading = ref(false)
const items = ref<KnowledgeBase[]>([])
const filterText = ref('')

const options = computed<KbOption[]>(() =>
  items.value.map(kb => ({
    label: kb.name,
    value: kb.id,
    docCount: kb.doc_count ?? 0,
    visibility: kb.visibility || 'private'
  }))
)

const filteredOptions = computed(() => {
  const q = filterText.value.trim().toLowerCase()
  if (!q) return options.value
  return options.value.filter(o =>
    o.label.toLowerCase().includes(q) || String(o.value).includes(q)
  )
})

function onFilter (val: string, update: (fn: () => void) => void) {
  update(() => { filterText.value = val })
}

async function load () {
  loading.value = true
  try {
    const { data } = await api.get<APIResponse<KnowledgeBase[]>>('/knowledge-base')
    items.value = (data.data ?? []) as KnowledgeBase[]
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => { void load() })
</script>
