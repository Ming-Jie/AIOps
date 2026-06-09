import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useRouter } from 'vue-router'
import { api } from 'boot/axios'
import type { KnowledgeBase } from 'src/api/types'
import { formatKbCount } from 'src/lib/knowledge/kb-utils'

export function useKnowledgeBasePage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const router = useRouter()

  const loading = ref(false)
  const items = ref<KnowledgeBase[]>([])
  const searchQuery = ref('')

  const dialogOpen = ref(false)
  const isEdit = ref(false)
  const editingId = ref<number | null>(null)
  const form = ref<{ name: string; description: string; visibility: string }>({
    name: '',
    description: '',
    visibility: 'private'
  })

  const filteredItems = computed(() => {
    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return items.value
    return items.value.filter(kb =>
      kb.name.toLowerCase().includes(q) ||
      (kb.description || '').toLowerCase().includes(q) ||
      String(kb.id).includes(q)
    )
  })

  const summary = computed(() => {
    const list = items.value
    return {
      kbCount: list.length,
      docCount: list.reduce((n, kb) => n + (kb.doc_count ?? 0), 0),
      publicCount: list.filter(kb => kb.visibility === 'public').length
    }
  })

  const load = async () => {
    loading.value = true
    try {
      const { data } = await api.get<{ data: KnowledgeBase[] }>('/knowledge-base')
      items.value = data.data || []
    } finally {
      loading.value = false
    }
  }

  const openCreate = () => {
    isEdit.value = false
    editingId.value = null
    form.value = { name: '', description: '', visibility: 'private' }
    dialogOpen.value = true
  }

  const openEdit = (kb: KnowledgeBase) => {
    isEdit.value = true
    editingId.value = kb.id
    form.value = { name: kb.name, description: kb.description || '', visibility: kb.visibility || 'private' }
    dialogOpen.value = true
  }

  const save = async () => {
    if (!form.value.name.trim()) {
      $q.notify({ type: 'warning', message: t('kbNameRequired') })
      return
    }
    try {
      if (isEdit.value && editingId.value != null) {
        await api.put(`/knowledge-base/${editingId.value}`, form.value)
      } else {
        await api.post('/knowledge-base', form.value)
      }
      dialogOpen.value = false
      void load()
      $q.notify({ type: 'positive', message: t('saveSuccess') || '保存成功' })
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '保存失败' })
    }
  }

  const remove = (kb: KnowledgeBase) => {
    const docCount = kb.doc_count ?? 0
    if (docCount > 0) {
      $q.dialog({
        title: t('confirm'),
        message: t('kbDeleteBlocked', { name: kb.name, n: docCount }),
        ok: { label: t('ok') || '知道了', flat: true, color: 'primary' },
        cancel: false
      })
      return
    }

    $q.dialog({
      title: t('confirm'),
      message: t('kbDeleteConfirm', { name: kb.name }),
      cancel: true,
      persistent: true
    }).onOk(async () => {
      try {
        await api.delete(`/knowledge-base/${kb.id}`)
        void load()
        $q.notify({ type: 'positive', message: t('deleteSuccess') || '删除成功' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message || '删除失败' })
      }
    })
  }

  const openDetail = (kb: KnowledgeBase) => {
    void router.push({ name: 'knowledge-detail', params: { id: String(kb.id) } })
  }

  return {
    t,
    loading,
    items,
    filteredItems,
    searchQuery,
    summary,
    formatKbCount,
    dialogOpen,
    isEdit,
    form,
    load,
    openCreate,
    openEdit,
    save,
    remove,
    openDetail
  }
}
