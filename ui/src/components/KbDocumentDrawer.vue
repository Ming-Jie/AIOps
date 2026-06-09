<template>
  <q-dialog
    :model-value="open"
    position="right"
    seamless
    @update:model-value="onOpenChange"
  >
    <q-card class="kb-doc-panel column no-wrap">
      <q-card-section class="row items-start q-pb-sm panel-header">
        <div class="col min-width-0">
          <div class="text-subtitle2 text-weight-medium ellipsis">{{ doc?.filename || t('kbDocDetail') }}</div>
          <div v-if="doc" class="row q-gutter-xs q-mt-xs items-center">
            <q-badge :color="statusMeta(doc.status).color" :label="statusMeta(doc.status).label" />
            <span class="text-caption text-grey-6">{{ formatSize(doc.size) }}</span>
          </div>
        </div>
        <q-btn flat round dense size="sm" icon="close" class="text-grey-7" @click="onOpenChange(false)" />
      </q-card-section>

      <q-card-section
        v-if="doc && (doc.status === 'failed' || doc.status === 'indexing' || doc.status === 'pending')"
        class="q-pt-none q-pb-sm"
      >
        <div class="status-note" :class="`status-note--${doc.status}`">
          <q-spinner v-if="doc.status === 'indexing'" color="primary" size="14px" />
          <q-icon v-else-if="doc.status === 'failed'" name="error_outline" size="14px" />
          <q-icon v-else name="schedule" size="14px" />
          <span>{{ bannerTitle(doc.status) }}</span>
          <span v-if="doc.error" class="status-note__detail">{{ doc.error }}</span>
        </div>
      </q-card-section>

      <q-card-section v-if="doc" class="col scroll q-pt-none panel-body">
        <div v-if="previewLoading" class="preview-placeholder">
          <q-spinner color="grey-6" size="20px" />
          <span class="text-caption text-grey-6 q-ml-sm">{{ t('kbPreviewLoading') }}</span>
        </div>

        <div v-else-if="previewError" class="preview-placeholder preview-placeholder--muted">
          <q-icon name="info_outline" size="18px" color="grey-6" />
          <span class="text-caption text-grey-7 q-ml-sm">{{ previewError }}</span>
        </div>

        <div v-else-if="previewKind === 'pdf' && previewUrl" class="preview-frame-wrap">
          <iframe :src="previewUrl" :title="doc.filename" class="preview-iframe" />
        </div>

        <div v-else-if="previewKind === 'markdown'" class="preview-md">
          <MdRenderer :content="previewText" />
        </div>

        <pre v-else-if="previewKind === 'text'" class="preview-text">{{ previewText }}</pre>

        <div v-else class="preview-placeholder preview-placeholder--muted">
          <span class="text-caption text-grey-6">{{ t('kbDocPreviewUnsupported') }}</span>
        </div>

        <q-expansion-item
          dense
          expand-separator
          icon="info_outline"
          :label="t('kbDocMeta')"
          header-class="text-caption text-grey-7 meta-expansion"
          class="q-mt-md"
        >
          <div class="meta-block">
            <div class="meta-row">
              <span class="meta-label">{{ t('kbDocUploadedAt') }}</span>
              <span>{{ new Date(doc.created_at).toLocaleString() }}</span>
            </div>
            <div v-if="doc.viking_uri" class="meta-row">
              <span class="meta-label">{{ t('kbDocUri') }}</span>
              <span class="mono">{{ doc.viking_uri }}</span>
            </div>
          </div>
        </q-expansion-item>
      </q-card-section>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { ref, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from 'boot/axios'
import MdRenderer from 'components/MdRenderer.vue'
import type { KBDocument } from 'src/api/types'

const props = defineProps<{
  open: boolean
  doc: KBDocument | null
  kbId: number
}>()

const emit = defineEmits<{ 'update:open': [v: boolean] }>()
const { t } = useI18n()

type PreviewKind = 'empty' | 'pdf' | 'markdown' | 'text' | 'unsupported'

const previewLoading = ref(false)
const previewError = ref('')
const previewKind = ref<PreviewKind>('empty')
const previewUrl = ref('')
const previewText = ref('')

function onOpenChange (v: boolean) {
  if (!v) resetPreview()
  emit('update:open', v)
}

function resetPreview () {
  revokePreviewUrl()
  previewLoading.value = false
  previewError.value = ''
  previewKind.value = 'empty'
  previewUrl.value = ''
  previewText.value = ''
}

function revokePreviewUrl () {
  if (previewUrl.value) {
    URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = ''
  }
}

function fileExt (name: string): string {
  const i = name.lastIndexOf('.')
  return i >= 0 ? name.slice(i).toLowerCase() : ''
}

function classifyPreview (filename: string, contentType: string): PreviewKind {
  const ext = fileExt(filename)
  const ct = contentType.toLowerCase()
  if (ext === '.pdf' || ct.includes('pdf')) return 'pdf'
  if (['.md', '.markdown'].includes(ext) || ct.includes('markdown')) return 'markdown'
  if (['.txt', '.text', '.html', '.htm', '.csv', '.json'].includes(ext) || ct.startsWith('text/')) return 'text'
  return 'unsupported'
}

async function loadPreview () {
  if (!props.doc?.id || !props.kbId) return
  resetPreview()
  previewLoading.value = true
  try {
    const res = await api.get(
      `/knowledge-base/${props.kbId}/documents/${props.doc.id}/preview`,
      { responseType: 'blob' }
    )
    const blob = res.data as Blob
    const contentType = String(res.headers['content-type'] || blob.type || '')
    const kind = classifyPreview(props.doc.filename, contentType)
    previewKind.value = kind
    if (kind === 'pdf') {
      previewUrl.value = URL.createObjectURL(blob)
    } else if (kind === 'markdown' || kind === 'text') {
      previewText.value = await blob.text()
    }
  } catch (e: unknown) {
    const err = e as { response?: { data?: Blob | { message?: string } } }
    previewError.value = t('kbPreviewFailed')
    if (err.response?.data instanceof Blob) {
      try {
        const text = await err.response.data.text()
        const parsed = JSON.parse(text) as { message?: string }
        if (parsed.message) previewError.value = parsed.message
      } catch { /* ignore */ }
    } else if (err.response?.data && typeof err.response.data === 'object' && 'message' in err.response.data) {
      previewError.value = (err.response.data as { message?: string }).message || previewError.value
    }
    previewKind.value = 'unsupported'
  } finally {
    previewLoading.value = false
  }
}

watch(
  () => [props.open, props.doc?.id, props.kbId] as const,
  ([open]) => {
    if (open && props.doc?.id) void loadPreview()
    if (!open) resetPreview()
  }
)

onUnmounted(resetPreview)

function formatSize (bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let n = bytes
  let i = 0
  while (n >= 1024 && i < units.length - 1) { n /= 1024; i++ }
  return `${n.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function statusMeta (status: string): { color: string; label: string } {
  switch (status) {
    case 'indexed': return { color: 'positive', label: t('kbStatusIndexed') }
    case 'indexing': return { color: 'orange', label: t('kbStatusIndexing') }
    case 'failed': return { color: 'negative', label: t('kbStatusFailed') }
    default: return { color: 'grey', label: t('kbStatusPending') }
  }
}

function bannerTitle (status: string): string {
  switch (status) {
    case 'failed': return t('kbStatusFailed')
    case 'indexing': return t('kbStatusIndexing')
    default: return t('kbStatusPending')
  }
}
</script>

<style scoped>
.kb-doc-panel {
  width: min(420px, calc(100vw - 24px));
  max-height: min(82vh, 720px);
  border-radius: 12px 0 0 12px;
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.08);
}
.panel-header {
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}
.panel-body {
  min-height: 0;
}
.min-width-0 {
  min-width: 0;
}
.status-note {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 12px;
  color: #616161;
  background: rgba(0, 0, 0, 0.03);
}
.status-note--failed {
  color: #c62828;
  background: rgba(198, 40, 40, 0.06);
}
.status-note--indexing {
  color: #1565c0;
  background: rgba(21, 101, 192, 0.06);
}
.status-note__detail {
  flex: 1 1 100%;
  font-size: 11px;
  opacity: 0.9;
  word-break: break-word;
}
.preview-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  border-radius: 8px;
  border: 1px dashed rgba(0, 0, 0, 0.1);
  background: rgba(0, 0, 0, 0.02);
}
.preview-placeholder--muted {
  min-height: 80px;
}
.preview-frame-wrap {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  overflow: hidden;
  background: #fafafa;
}
.preview-iframe {
  display: block;
  width: 100%;
  height: min(52vh, 480px);
  border: 0;
  background: #fff;
}
.preview-md {
  max-height: min(52vh, 480px);
  overflow: auto;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(0, 0, 0, 0.02);
  font-size: 13px;
}
.preview-text {
  max-height: min(52vh, 480px);
  overflow: auto;
  margin: 0;
  padding: 10px 12px;
  border-radius: 8px;
  border: 1px solid rgba(0, 0, 0, 0.08);
  background: rgba(0, 0, 0, 0.02);
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 12px;
  line-height: 1.5;
  color: #424242;
}
.meta-expansion :deep(.q-item) {
  min-height: 36px;
  padding-left: 0;
  padding-right: 0;
}
.meta-block {
  padding: 0 4px 8px;
}
.meta-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-bottom: 10px;
  font-size: 12px;
  color: #616161;
}
.meta-label {
  font-size: 11px;
  color: #9e9e9e;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  word-break: break-all;
}
</style>
