<template>
  <q-dialog v-model="internalOpen" persistent>
    <q-card class="checklist-card">
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">工作流检查清单</div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup @click="$emit('close')" />
      </q-card-section>

      <q-card-section>
        <q-banner
          v-if="passedCount === items.length"
          class="bg-positive text-white q-mb-md"
          dense
        >
          <template #avatar>
            <q-icon name="check_circle" />
          </template>
          所有检查通过，可以保存/执行
        </q-banner>

        <q-banner
          v-else
          class="bg-warning text-white q-mb-md"
          dense
        >
          <template #avatar>
            <q-icon name="warning" />
          </template>
          {{ failedCount }} 项未通过，建议修复后再保存/执行
        </q-banner>

        <q-list dense>
          <q-item v-for="(item, idx) in items" :key="idx" class="checklist-item">
            <q-item-section avatar>
              <q-icon
                :name="item.passed ? 'check_circle' : 'error_outline'"
                :color="item.passed ? 'positive' : 'negative'"
                size="20px"
              />
            </q-item-section>
            <q-item-section>
              <q-item-label>{{ item.message }}</q-item-label>
              <q-item-label v-if="!item.passed && item.hint" caption class="text-negative">
                {{ item.hint }}
              </q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
      </q-card-section>

      <q-card-actions align="right">
        <q-btn flat label="关闭" v-close-popup @click="$emit('close')" />
        <q-btn
          v-if="allowContinue"
          color="primary"
          label="忽略并继续"
          v-close-popup
          @click="$emit('continue')"
        />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export interface ChecklistItem {
  message: string
  hint?: string
  passed: boolean
}

const props = defineProps<{
  modelValue: boolean
  items: ChecklistItem[]
  allowContinue?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
  close: []
  continue: []
}>()

const internalOpen = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v)
})

const passedCount = computed(() => props.items.filter(i => i.passed).length)
const failedCount = computed(() => props.items.filter(i => !i.passed).length)
</script>

<style scoped>
.checklist-card {
  width: min(520px, 92vw);
  max-width: 92vw;
}

.checklist-item {
  border-bottom: 1px solid #f5f5f5;
}

.checklist-item:last-child {
  border-bottom: none;
}
</style>
