<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <q-btn flat round dense icon="arrow_back" @click="goBack" class="q-mr-sm" />
      <div class="col">
        <div class="text-h6">{{ kb?.name || t('knowledgeDetail') }}</div>
        <div v-if="kb?.description" class="text-caption text-grey-7">{{ kb.description }}</div>
      </div>
      <q-btn flat round dense icon="refresh" @click="loadDocuments" :loading="loadingDocs">
        <q-tooltip>{{ t('refresh') }}</q-tooltip>
      </q-btn>
      <q-btn
        v-if="canManage"
        flat
        round
        dense
        icon="delete"
        :color="(kb?.doc_count ?? documents.length) > 0 ? 'grey-5' : 'negative'"
        class="q-ml-xs"
        @click="removeKb"
      >
        <q-tooltip>{{ (kb?.doc_count ?? documents.length) > 0 ? t('kbDeleteBlockedShort') : t('delete') }}</q-tooltip>
      </q-btn>
    </div>

    <div v-if="documents.length > 0" class="row q-col-gutter-sm q-mb-md">
      <div class="col-4">
        <q-card flat bordered>
          <q-card-section class="q-py-sm">
            <div class="text-caption text-grey-7">{{ t('kbStatDocs') }}</div>
            <div class="text-subtitle1">{{ docSummary.total }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-4">
        <q-card flat bordered>
          <q-card-section class="q-py-sm">
            <div class="text-caption text-grey-7">{{ t('kbStatusIndexed') }}</div>
            <div class="text-subtitle1 text-positive">{{ docSummary.indexed }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-4">
        <q-card flat bordered>
          <q-card-section class="q-py-sm">
            <div class="text-caption text-grey-7">{{ t('kbStatusFailed') }}</div>
            <div class="text-subtitle1" :class="docSummary.failed ? 'text-negative' : ''">{{ docSummary.failed }}</div>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <q-banner v-if="hasIndexing" dense rounded class="bg-blue-1 q-mb-md">
      <template #avatar><q-spinner color="primary" size="20px" /></template>
      {{ t('kbIndexingBanner', { n: indexingCount }) }}
    </q-banner>

    <q-banner v-if="failedDocs.length > 0" dense rounded class="bg-red-1 q-mb-md">
      <template #avatar><q-icon name="error" color="negative" /></template>
      <div>{{ t('kbFailedBanner', { n: failedDocs.length }) }}</div>
      <div class="text-caption q-mt-xs">
        <span v-for="(doc, i) in failedDocs.slice(0, 3)" :key="doc.id">
          {{ doc.filename }}<span v-if="doc.error">: {{ doc.error }}</span><span v-if="i < Math.min(failedDocs.length, 3) - 1"> · </span>
        </span>
        <span v-if="failedDocs.length > 3"> …</span>
      </div>
    </q-banner>

    <q-tabs v-model="tab" align="left" class="text-primary q-mb-md" narrow-indicator>
      <q-tab name="documents" :label="t('kbTabDocuments')" icon="description" />
      <q-tab name="search" :label="t('kbTabSearch')" icon="search" />
    </q-tabs>

    <q-tab-panels v-model="tab" animated keep-alive>
      <q-tab-panel name="documents" class="q-pa-none">
        <template v-if="canManage">
          <q-uploader
            ref="uploaderRef"
            hide-upload-btn
            multiple
            :label="t('kbUploadLabel')"
            class="q-mb-md full-width"
            accept=".pdf,.md,.markdown,.txt,.html,.htm,.docx,.epub,.xlsx,.xls,.pptx,.csv,.json"
            @added="onFilesAdded"
          />
          <div class="text-caption text-grey-6 q-mb-sm">{{ t('kbUploadHint') }}</div>
          <div class="row items-center q-gutter-sm q-mb-md">
            <q-btn
              outline
              color="primary"
              icon="link"
              :label="t('kbImportUrlBtn')"
              no-caps
              @click="openImportDialog"
            />
          </div>
        </template>
        <q-banner v-else dense class="bg-grey-2 text-grey-8 q-mb-md rounded-borders">
          <template #avatar><q-icon name="visibility" /></template>
          {{ t('kbReadonlyHint') }}
        </q-banner>

        <q-table
          :rows="documents"
          :columns="docColumns"
          row-key="id"
          :loading="loadingDocs"
          flat
          :no-data-label="t('kbNoDocs')"
        >
          <template #body-cell-filename="props">
            <q-td :props="props">
              <span
                class="doc-name-link cursor-pointer text-primary"
                @click="openDocDrawer(props.row)"
              >
                {{ props.row.filename }}
              </span>
            </q-td>
          </template>
          <template #body-cell-size="props">
            <q-td :props="props">{{ formatSize(props.row.size) }}</q-td>
          </template>
          <template #body-cell-status="props">
            <q-td :props="props">
              <div class="row items-center justify-center no-wrap q-gutter-xs">
                <q-spinner v-if="props.row.status === 'indexing'" color="orange" size="16px" />
                <q-badge :color="statusMeta(props.row.status).color" :label="statusMeta(props.row.status).label" />
                <q-btn
                  v-if="props.row.status === 'failed' && props.row.error"
                  flat round dense size="sm" icon="info" color="negative"
                  @click="openDocDrawer(props.row)"
                >
                  <q-tooltip>{{ props.row.error }}</q-tooltip>
                </q-btn>
              </div>
            </q-td>
          </template>
          <template #body-cell-created_at="props">
            <q-td :props="props">{{ new Date(props.row.created_at).toLocaleString() }}</q-td>
          </template>
          <template #body-cell-actions="props">
            <q-td :props="props">
              <q-btn v-if="canManage" flat color="negative" size="sm" :label="t('delete')" @click="removeDocument(props.row)" />
              <span v-else class="text-grey-5">—</span>
            </q-td>
          </template>
        </q-table>
      </q-tab-panel>

      <q-tab-panel name="search" class="q-pa-none">
        <div class="row items-center q-gutter-sm q-mb-md">
          <q-input
            v-model="query"
            outlined
            dense
            class="col"
            :placeholder="t('kbQueryPlaceholder')"
            @keyup.enter="runSearch"
          />
          <q-input v-model.number="topK" outlined dense type="number" style="width: 90px" label="top_k" />
          <q-btn color="primary" icon="search" :label="t('kbSearchBtn')" :loading="searching" @click="runSearch" unelevated />
        </div>

        <div v-if="searched && !searching && enrichedHits.length === 0" class="text-grey-6 q-pa-lg text-center">
          <q-icon name="search_off" size="40px" class="q-mb-sm" />
          <div>{{ t('kbNoHits') }}</div>
        </div>

        <q-list bordered separator v-if="enrichedHits.length > 0">
          <q-item v-for="(hit, idx) in enrichedHits" :key="idx">
            <q-item-section>
              <q-item-label class="row items-center q-gutter-xs">
                <q-badge color="primary" :label="`${(hit.score * 100).toFixed(1)}%`" />
                <span class="text-weight-medium">{{ hit.docName || t('kbHitUnknownDoc') }}</span>
                <q-badge v-if="hit.isSummary" outline color="orange" :label="t('kbHitSummaryOnly')" />
              </q-item-label>
              <q-item-label caption class="mono ellipsis q-mt-xs">{{ hit.uri }}</q-item-label>
              <q-item-label class="q-mt-sm kb-hit-text">{{ hit.text || '—' }}</q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
      </q-tab-panel>
    </q-tab-panels>

    <KbDocumentDrawer
      v-model:open="drawerOpen"
      :doc="selectedDoc"
      :kb-id="kbId"
    />

    <KbUrlImportDialog
      v-model:open="importDialogOpen"
      :kb-id="kbId"
      @imported="onUrlsImported"
    />
  </q-page>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { QUploader } from 'quasar'
import KbDocumentDrawer from 'components/KbDocumentDrawer.vue'
import KbUrlImportDialog from 'components/KbUrlImportDialog.vue'
import { useKnowledgeBaseDetailPage } from './useKnowledgeBaseDetailPage'

defineOptions({ name: 'KnowledgeBaseDetailPage' })

const {
  t, kbId, tab, kb, canManage, documents, loadingDocs, uploading,
  importDialogOpen,
  query, topK, searching, enrichedHits, searched,
  drawerOpen, selectedDoc, docSummary, hasIndexing, indexingCount, failedDocs,
  docColumns, formatSize, statusMeta,
  loadDocuments, uploadFile, openImportDialog, onUrlsImported,
  removeDocument, removeKb, openDocDrawer, runSearch, goBack, init
} = useKnowledgeBaseDetailPage()

void uploading

const uploaderRef = ref<QUploader | null>(null)

const onFilesAdded = async (files: readonly File[]) => {
  await uploadFile(files)
  uploaderRef.value?.reset()
}

onMounted(init)
</script>

<style scoped>
.kb-hit-text {
  white-space: pre-wrap;
  word-break: break-word;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
.doc-name-link:hover {
  text-decoration: underline;
}
</style>
