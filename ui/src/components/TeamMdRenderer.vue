<template>
  <!-- eslint-disable-next-line vue/no-v-html -->
  <div class="chat-md-root" v-html="html" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { renderTeamChatMarkdown } from 'src/utils/teamChatMarkdown'

const props = defineProps<{
  content: string
}>()

const html = computed(() => renderTeamChatMarkdown(props.content))
</script>
