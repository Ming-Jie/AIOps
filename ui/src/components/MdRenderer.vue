<template>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="chat-md-root" v-html="html" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { renderChatMarkdown } from 'src/utils/chatMarkdown'

const props = defineProps<{
  content: string
}>()

const html = computed(() => renderChatMarkdown(props.content))
</script>
