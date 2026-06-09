<template>
  <q-dialog v-model="internalOpen">
    <q-card class="sysvars-card">
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">系统变量参考</div>
        <q-space />
        <q-btn icon="close" flat round dense v-close-popup />
      </q-card-section>

      <q-card-section>
        <p class="text-caption text-grey-7 q-mb-md">
          在节点配置中使用 <code>\{\{变量名\}\}</code> 引用变量
        </p>

        <q-table
          flat
          dense
          :rows="systemVariables"
          :columns="columns"
          row-key="name"
          hide-pagination
          :rows-per-page-options="[0]"
        >
          <template #body-cell-example="cell">
            <q-td :props="cell">
              <code class="var-code">{{ cell.row.example }}</code>
            </q-td>
          </template>
        </q-table>

        <q-separator class="q-my-md" />

        <div class="text-subtitle2 q-mb-sm">引用上游节点输出</div>
        <p class="text-caption text-grey-7">
          使用 <code>\{\{.{{ '{' }}节点标签}}.{{ '{' }}字段名}}\}</code> 引用上游节点的输出字段。
        </p>
        <p class="text-caption text-grey-7">
          例如: <code>\{\{.天气查询.temperature}}</code>
        </p>
      </q-card-section>

      <q-card-actions align="right">
        <q-btn flat label="关闭" v-close-popup />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
}>()

const internalOpen = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v)
})

const columns = [
  { name: 'name', label: '变量', field: 'name', align: 'left' as const },
  { name: 'description', label: '说明', field: 'description', align: 'left' as const },
  { name: 'example', label: '引用方式', field: 'example', align: 'left' as const }
]

const systemVariables = [
  { name: 'sys.query', description: '用户输入内容', example: '{{sys.query}}' },
  { name: 'sys.workflow_id', description: '当前工作流 ID', example: '{{sys.workflow_id}}' },
  { name: 'sys.workflow_run_id', description: '当前运行 ID', example: '{{sys.workflow_run_id}}' },
  { name: 'sys.user_id', description: '当前用户 ID', example: '{{sys.user_id}}' },
  { name: 'last_output', description: '上一步节点完整输出', example: '{{last_output}}' },
  { name: 'last_output.json', description: '上一步节点 JSON 输出', example: '{{last_output.json}}' },
  { name: 'last_output.items', description: '上一步节点 Items 数组', example: '{{last_output.items}}' },
  { name: '$items', description: '批量数据数组', example: '{{$items}}' },
  { name: '$first', description: '批处理第一个元素', example: '{{$first.json}}' }
]
</script>

<style scoped>
.sysvars-card {
  width: min(640px, 92vw);
  max-width: 92vw;
}

.var-code {
  background: #f5f5f5;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 12px;
  font-family: monospace;
  color: #1890ff;
}

code {
  background: #f5f5f5;
  padding: 2px 6px;
  border-radius: 3px;
  font-family: monospace;
  font-size: 12px;
  color: #333;
}
</style>
