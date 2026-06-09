<template>
  <q-dialog :model-value="open" persistent @update:model-value="onOpenChange">
    <q-card class="kb-url-import-card">
      <q-card-section>
        <div class="text-h6">{{ t('kbImportDialogTitle') }}</div>
        <div class="text-caption text-grey-7 q-mt-xs">{{ t('kbImportDialogDesc') }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none">
        <q-input
          v-model="urlText"
          type="textarea"
          outlined
          autogrow
          :rows="6"
          :placeholder="t('kbImportDialogPh')"
          :disable="importing"
          class="url-textarea"
        />
        <div class="text-caption text-grey-6 q-mt-sm">{{ t('kbImportUrlHint') }}</div>

        <q-banner v-if="result" dense rounded class="q-mt-md" :class="resultBannerClass">
          <div>{{ resultSummary }}</div>
          <q-list v-if="result.failed.length > 0" dense class="q-mt-sm">
            <q-item v-for="(item, idx) in result.failed" :key="idx" class="q-px-none">
              <q-item-section>
                <q-item-label caption class="mono ellipsis">{{ item.url }}</q-item-label>
                <q-item-label class="text-negative">{{ item.message }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-banner>
      </q-card-section>

      <q-card-actions align="right">
        <q-btn flat :label="t('cancel')" :disable="importing" @click="close" />
        <q-btn
          color="primary"
          unelevated
          icon="download"
          :label="t('kbImportDialogSubmit')"
          :loading="importing"
          :disable="!urlText.trim()"
          @click="submit"
        />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from 'boot/axios'
import type { KBImportURLsResult } from 'src/api/types'

const props = defineProps<{
  open: boolean
  kbId: number
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  imported: []
}>()

const { t } = useI18n()

const urlText = ref('')
const importing = ref(false)
const result = ref<KBImportURLsResult | null>(null)

const parseUrls = (text: string): string[] =>
  text
    .split(/\r?\n/)
    .map(line => line.trim())
    .filter(Boolean)

const resultSummary = computed(() => {
  if (!result.value) return ''
  const ok = result.value.imported.length
  const fail = result.value.failed.length
  if (fail === 0) return t('kbImportDialogAllOk', { n: ok })
  if (ok === 0) return t('kbImportDialogAllFailed', { n: fail })
  return t('kbImportDialogPartial', { ok, fail })
})

const resultBannerClass = computed(() => {
  if (!result.value) return ''
  if (result.value.failed.length === 0) return 'bg-green-1'
  if (result.value.imported.length === 0) return 'bg-red-1'
  return 'bg-orange-1'
})

const reset = () => {
  urlText.value = ''
  result.value = null
  importing.value = false
}

watch(() => props.open, (val) => {
  if (val) reset()
})

const onOpenChange = (val: boolean) => {
  if (!val && !importing.value) {
    if (result.value && result.value.imported.length > 0) {
      emit('imported')
    }
    emit('update:open', false)
  }
}

const close = () => {
  if (importing.value) return
  onOpenChange(false)
}

const submit = async () => {
  const urls = parseUrls(urlText.value)
  if (urls.length === 0) {
    return
  }
  const invalid = urls.find(u => !/^https:\/\/.+/i.test(u))
  if (invalid) {
    result.value = {
      imported: [],
      failed: [{ url: invalid, message: t('kbImportUrlHttpsOnly') }]
    }
    return
  }

  importing.value = true
  result.value = null
  try {
    const { data } = await api.post<{ data: KBImportURLsResult }>(
      `/knowledge-base/${props.kbId}/documents/import-urls`,
      { urls }
    )
    result.value = data.data || { imported: [], failed: [] }
    if (result.value.imported.length > 0 && result.value.failed.length === 0) {
      urlText.value = ''
    }
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    result.value = {
      imported: [],
      failed: [{ url: urls[0], message: err.response?.data?.message || t('kbImportUrlFailed') }]
    }
  } finally {
    importing.value = false
  }
}
</script>

<style scoped>
.kb-url-import-card {
  width: 560px;
  max-width: 92vw;
}
.url-textarea :deep(textarea) {
  min-height: 140px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 13px;
}
.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}
</style>
