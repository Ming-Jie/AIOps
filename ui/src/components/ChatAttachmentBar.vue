<template>
  <div v-if="hasItems" class="chat-attachment-bar q-mt-sm">
    <div
      v-for="(src, idx) in images"
      :key="'img-' + idx"
      class="chat-attachment-bar-image-wrap q-mb-sm"
    >
      <img
        :src="src"
        class="chat-attachment-bar-image cursor-pointer"
        alt=""
        loading="lazy"
        @click="emit('preview-image', src)"
      >
    </div>
    <div v-if="files.length > 0" class="row q-gutter-x-sm q-gutter-y-sm">
      <q-btn
        v-for="(att, fi) in files"
        :key="'file-' + fi"
        :label="fileButtonLabel(att)"
        :href="attachmentDownloadHref(att)"
        :download="att.filename"
        dense
        no-caps
        size="sm"
        color="secondary"
        outline
        icon="download"
        rel="noopener"
        target="_blank"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { FileAttachment } from 'src/api/types'
import { attachmentDownloadHref, formatAttachmentSize } from 'src/utils/chatAttachments'

const props = defineProps<{
  files?: FileAttachment[]
  images?: string[]
}>()

const emit = defineEmits<{
  'preview-image': [src: string]
}>()

const files = computed(() => props.files ?? [])
const images = computed(() => props.images ?? [])
const hasItems = computed(() => files.value.length > 0 || images.value.length > 0)

function fileButtonLabel (att: FileAttachment): string {
  const size = formatAttachmentSize(att.size)
  return size ? `${att.filename} (${size})` : att.filename
}
</script>

<style scoped>
.chat-attachment-bar-image {
  max-width: min(100%, 420px);
  max-height: 320px;
  border-radius: 8px;
  display: block;
}
</style>
