<template>
  <q-menu
    ref="menuRef"
    context-menu
    touch-position
    @before-hide="$emit('close')"
  >
    <q-list dense style="min-width: 140px">
      <q-item-label header class="menu-header">
        <div class="menu-header-row">
          <q-icon :name="nodeIcon" size="14px" class="q-mr-xs" />
          <span class="menu-node-label">{{ nodeLabel }}</span>
        </div>
      </q-item-label>

      <q-item clickable v-close-popup @click="$emit('edit')">
        <q-item-section avatar>
          <q-icon name="edit" size="14px" />
        </q-item-section>
        <q-item-section>编辑</q-item-section>
        <q-item-section side>
          <span class="menu-shortcut">双击</span>
        </q-item-section>
      </q-item>

      <q-item clickable v-close-popup @click="$emit('rename')">
        <q-item-section avatar>
          <q-icon name="edit_note" size="14px" />
        </q-item-section>
        <q-item-section>重命名</q-item-section>
      </q-item>

      <q-item clickable v-close-popup @click="$emit('duplicate')">
        <q-item-section avatar>
          <q-icon name="content_copy" size="14px" />
        </q-item-section>
        <q-item-section>复制</q-item-section>
      </q-item>

      <q-separator />

      <q-item clickable v-close-popup @click="$emit('disable')" v-if="canDisable">
        <q-item-section avatar>
          <q-icon name="visibility_off" size="14px" />
        </q-item-section>
        <q-item-section>禁用</q-item-section>
      </q-item>

      <q-item clickable v-close-popup @click="$emit('delete')" class="menu-delete">
        <q-item-section avatar>
          <q-icon name="delete" size="14px" color="negative" />
        </q-item-section>
        <q-item-section class="text-negative">删除</q-item-section>
        <q-item-section side>
          <span class="menu-shortcut">Del</span>
        </q-item-section>
      </q-item>
    </q-list>
  </q-menu>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { getNodeIcon } from 'src/lib/upstreamOutputs'

const props = defineProps<{
  nodeType: string
  nodeLabel: string
  canDisable?: boolean
}>()

defineEmits<{
  edit: []
  rename: []
  duplicate: []
  delete: []
  disable: []
  close: []
}>()

const menuRef = ref(null)

const nodeIcon = computed(() => getNodeIcon(props.nodeType))
</script>

<style scoped>
.menu-header {
  padding: 8px 12px;
  font-size: 12px;
  color: #8c8c8c;
}

.menu-header-row {
  display: flex;
  align-items: center;
}

.menu-node-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.menu-shortcut {
  font-size: 10px;
  color: #bfbfbf;
  margin-left: 12px;
}

.menu-delete {
  border-top: 1px solid #f0f0f0;
  margin-top: 4px;
}
</style>
