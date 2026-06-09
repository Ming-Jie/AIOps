import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { api } from 'boot/axios'
import type { APIResponse, ModelConfig } from 'src/api/types'

const defaultForm = {
  name: '',
  provider: 'openai',
  model: '',
  base_url: '',
  api_key: '',
  is_active: true,
  purpose: 'chat'
}

export function useModelsPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const loading = ref(false)
  const saving = ref(false)
  const rows = ref<ModelConfig[]>([])
  const errorMsg = ref('')
  const dialogOpen = ref(false)
  const editingId = ref<number | null>(null)
  const form = reactive({ ...defaultForm })
  const showApiKey = ref(false)

  const providerOptions = computed(() => [
    { label: 'OpenAI', value: 'openai' },
    { label: 'Ark (火山引擎)', value: 'ark' }
  ])

  const purposeOptions = computed(() => [
    { label: t('modelPurposeChat'), value: 'chat' },
    { label: t('modelPurposeEmbedding'), value: 'embedding' }
  ])

  const providerColor = (provider: string): string => {
    switch (provider) {
      case 'openai': return 'blue'
      case 'ark': return 'purple'
      default: return 'grey'
    }
  }

  const columns = computed(() => [
    { name: 'id', label: 'ID', field: 'id', align: 'left' as const },
    { name: 'name', label: t('modelName'), field: 'name', align: 'left' as const },
    { name: 'purpose', label: t('modelPurpose'), field: 'purpose', align: 'left' as const },
    { name: 'provider', label: t('modelProvider'), field: 'provider', align: 'left' as const },
    { name: 'model', label: t('modelModel'), field: 'model', align: 'left' as const },
    { name: 'base_url', label: t('modelBaseUrl'), field: 'base_url', align: 'left' as const },
    { name: 'is_active', label: t('status'), field: 'is_active', align: 'center' as const },
    { name: 'actions', label: t('actions'), field: 'actions', align: 'right' as const }
  ])

  async function load () {
    loading.value = true
    errorMsg.value = ''
    try {
      const { data } = await api.get<APIResponse<ModelConfig[]>>('/models')
      rows.value = (data.data ?? []) as ModelConfig[]
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      errorMsg.value = err.response?.data?.message ?? t('loadFailed')
      rows.value = []
    } finally {
      loading.value = false
    }
  }

  function openDialog (model?: ModelConfig) {
    showApiKey.value = false
    if (model) {
      editingId.value = model.id
      form.name = model.name
      form.provider = model.provider
      form.model = model.model
      form.base_url = model.base_url || ''
      form.api_key = ''
      form.is_active = model.is_active !== false
      form.purpose = model.purpose || 'chat'
    } else {
      editingId.value = null
      Object.assign(form, defaultForm)
    }
    dialogOpen.value = true
  }

  async function saveModel () {
    if (!form.name || !form.model) return
    saving.value = true
    try {
      const body: Record<string, unknown> = {
        name: form.name,
        provider: form.provider,
        model: form.model,
        base_url: form.base_url || undefined,
        is_active: form.is_active,
        purpose: form.purpose
      }
      if (form.api_key) {
        body.api_key = form.api_key
      }

      if (editingId.value) {
        await api.put(`/models/${editingId.value}`, body)
        $q.notify({ type: 'positive', message: t('updateOk') })
      } else {
        await api.post('/models', body)
        $q.notify({ type: 'positive', message: t('createOk') })
      }
      dialogOpen.value = false
      await load()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('operationFailed') })
    } finally {
      saving.value = false
    }
  }

  const confirmDelete = (model: ModelConfig) => {
    $q.dialog({
      title: t('confirmDelete'),
      message: `${model.name} (ID: ${model.id})`,
      cancel: { label: t('cancel'), flat: true },
      ok: { label: t('delete'), color: 'negative' }
    }).onOk(async () => {
      try {
        await api.delete(`/models/${model.id}`)
        $q.notify({ type: 'positive', message: t('deleteOk') })
        await load()
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message ?? t('deleteFailed') })
      }
    })
  }

  onMounted(() => { void load() })

  return {
    t,
    loading,
    saving,
    rows,
    errorMsg,
    columns,
    load,
    dialogOpen,
    editingId,
    form,
    showApiKey,
    openDialog,
    saveModel,
    confirmDelete,
    providerOptions,
    providerColor,
    purposeOptions
  }
}
