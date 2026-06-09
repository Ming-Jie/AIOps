import { ref, computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'
import { useRoute, useRouter } from 'vue-router'
import { api } from 'boot/axios'
import type { KnowledgeBase, KBDocument, KBSearchHit } from 'src/api/types'
import { isKbDocIndexing, parseDocNameFromUri } from 'src/lib/knowledge/kb-utils'

export const kbDocColumns = [
  { name: 'filename', label: '文件名', field: 'filename', align: 'left' as const },
  { name: 'size', label: '大小', field: 'size', align: 'right' as const },
  { name: 'status', label: '状态', field: 'status', align: 'center' as const },
  { name: 'created_at', label: '上传时间', field: 'created_at', align: 'left' as const },
  { name: 'actions', label: '操作', field: 'actions', align: 'center' as const }
]

const POLL_MS_INDEXING = 5000
const POLL_MS_IDLE = 3000

export function useKnowledgeBaseDetailPage () {
  const { t } = useI18n()
  const $q = useQuasar()
  const route = useRoute()
  const router = useRouter()

  const kbId = computed(() => Number(route.params.id))
  const tab = ref<'documents' | 'search'>('documents')

  const kb = ref<KnowledgeBase | null>(null)
  const canManage = computed(() => !!kb.value?.can_manage)
  const documents = ref<KBDocument[]>([])
  const loadingDocs = ref(false)
  const uploading = ref(false)
  const importDialogOpen = ref(false)

  const query = ref('')
  const topK = ref(10)
  const searching = ref(false)
  const hits = ref<KBSearchHit[]>([])
  const searched = ref(false)

  const drawerOpen = ref(false)
  const selectedDoc = ref<KBDocument | null>(null)

  let pollTimer: ReturnType<typeof setInterval> | null = null

  const formatSize = (bytes: number): string => {
    if (!bytes) return '0 B'
    const units = ['B', 'KB', 'MB', 'GB']
    let n = bytes
    let i = 0
    while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
    return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
  }

  const statusMeta = (status: string): { color: string; label: string; icon?: string } => {
    switch (status) {
      case 'indexed': return { color: 'positive', label: t('kbStatusIndexed'), icon: 'check_circle' }
      case 'indexing': return { color: 'orange', label: t('kbStatusIndexing'), icon: 'hourglass_top' }
      case 'failed': return { color: 'negative', label: t('kbStatusFailed'), icon: 'error' }
      default: return { color: 'grey', label: t('kbStatusPending'), icon: 'schedule' }
    }
  }

  const hasIndexing = computed(() => documents.value.some(d => isKbDocIndexing(d.status)))
  const failedDocs = computed(() => documents.value.filter(d => d.status === 'failed'))
  const indexingCount = computed(() => documents.value.filter(d => isKbDocIndexing(d.status)).length)

  const docSummary = computed(() => ({
    total: documents.value.length,
    indexed: documents.value.filter(d => d.status === 'indexed').length,
    failed: failedDocs.value.length
  }))

  const enrichedHits = computed(() =>
    hits.value.map(hit => ({
      ...hit,
      docName: parseDocNameFromUri(hit.uri),
      text: (hit.content || hit.abstract || hit.overview || '').trim(),
      isSummary: !hit.content && !!(hit.abstract || hit.overview)
    }))
  )

  const loadKB = async () => {
    try {
      const { data } = await api.get<{ data: KnowledgeBase[] }>('/knowledge-base')
      kb.value = (data.data || []).find(k => k.id === kbId.value) || null
    } catch { /* ignore */ }
  }

  const loadDocuments = async () => {
    loadingDocs.value = true
    try {
      const { data } = await api.get<{ data: KBDocument[] }>(`/knowledge-base/${kbId.value}/documents`, {
        params: { _t: Date.now() }
      })
      documents.value = data.data || []
      syncSelectedDoc()
      managePolling()
    } finally {
      loadingDocs.value = false
    }
  }

  const syncSelectedDoc = () => {
    if (!selectedDoc.value?.id) return
    const live = documents.value.find(d => d.id === selectedDoc.value?.id)
    if (live) selectedDoc.value = live
  }

  const managePolling = () => {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    if (!hasIndexing.value) return
    const interval = hasIndexing.value ? POLL_MS_INDEXING : POLL_MS_IDLE
    pollTimer = setInterval(() => { void refreshSilently() }, interval)
  }

  const refreshSilently = async () => {
    try {
      const { data } = await api.get<{ data: KBDocument[] }>(`/knowledge-base/${kbId.value}/documents`, {
        params: { _t: Date.now() }
      })
      documents.value = data.data || []
      syncSelectedDoc()
      if (!hasIndexing.value && pollTimer) {
        clearInterval(pollTimer)
        pollTimer = null
      }
    } catch { /* ignore */ }
  }

  const openDocDrawer = (doc: KBDocument) => {
    selectedDoc.value = doc
    drawerOpen.value = true
  }

  const uploadFile = async (files: readonly File[]) => {
    if (!files || files.length === 0) return
    uploading.value = true
    try {
      for (const file of files) {
        const fd = new FormData()
        fd.append('file', file)
        await api.post(`/knowledge-base/${kbId.value}/documents`, fd, {
          headers: { 'Content-Type': 'multipart/form-data' }
        })
      }
      $q.notify({ type: 'positive', message: t('kbUploadStarted') })
      void loadDocuments()
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '上传失败' })
    } finally {
      uploading.value = false
    }
  }

  const openImportDialog = () => { importDialogOpen.value = true }

  const onUrlsImported = () => {
    void loadDocuments()
    void loadKB()
  }

  const removeDocument = (doc: KBDocument) => {
    $q.dialog({
      title: t('confirm'),
      message: t('kbDocDeleteConfirm', { name: doc.filename }),
      cancel: true,
      persistent: true
    }).onOk(async () => {
      try {
        await api.delete(`/knowledge-base/${kbId.value}/documents/${doc.id}`)
        if (selectedDoc.value?.id === doc.id) {
          drawerOpen.value = false
          selectedDoc.value = null
        }
        void loadDocuments()
        void loadKB()
        $q.notify({ type: 'positive', message: t('deleteSuccess') || '删除成功' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message || '删除失败' })
      }
    })
  }

  const removeKb = () => {
    if (!kb.value) return
    const docCount = kb.value.doc_count ?? documents.value.length
    if (docCount > 0) {
      $q.dialog({
        title: t('confirm'),
        message: t('kbDeleteBlocked', { name: kb.value.name, n: docCount }),
        ok: { label: t('ok') || '知道了', flat: true, color: 'primary' },
        cancel: false
      })
      return
    }
    $q.dialog({
      title: t('confirm'),
      message: t('kbDeleteConfirm', { name: kb.value.name }),
      cancel: true,
      persistent: true
    }).onOk(async () => {
      try {
        await api.delete(`/knowledge-base/${kbId.value}`)
        $q.notify({ type: 'positive', message: t('deleteSuccess') || '删除成功' })
        void router.push({ name: 'knowledge' })
      } catch (e: unknown) {
        const err = e as { response?: { data?: { message?: string } } }
        $q.notify({ type: 'negative', message: err.response?.data?.message || '删除失败' })
      }
    })
  }

  const runSearch = async () => {
    if (!query.value.trim()) {
      $q.notify({ type: 'warning', message: t('kbQueryRequired') })
      return
    }
    searching.value = true
    searched.value = true
    try {
      const { data } = await api.post<{ data: KBSearchHit[] }>(`/knowledge-base/${kbId.value}/search`, {
        query: query.value,
        top_k: topK.value
      })
      hits.value = data.data || []
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } } }
      $q.notify({ type: 'negative', message: err.response?.data?.message || '检索失败' })
      hits.value = []
    } finally {
      searching.value = false
    }
  }

  const goBack = () => { void router.push({ name: 'knowledge' }) }

  const init = () => {
    void loadKB()
    void loadDocuments()
  }

  onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

  return {
    t,
    kbId,
    tab,
    kb,
    canManage,
    documents,
    loadingDocs,
    uploading,
    importDialogOpen,
    query,
    topK,
    searching,
    hits,
    enrichedHits,
    searched,
    drawerOpen,
    selectedDoc,
    docSummary,
    hasIndexing,
    indexingCount,
    failedDocs,
    docColumns: kbDocColumns,
    formatSize,
    statusMeta,
    loadDocuments,
    uploadFile,
    openImportDialog,
    onUrlsImported,
    removeDocument,
    removeKb,
    openDocDrawer,
    runSearch,
    goBack,
    init
  }
}
