<template>
  <q-dialog :model-value="open" @update:model-value="onOpenChange">
    <q-card style="min-width: 360px; max-width: 420px">
      <q-card-section>
        <div class="text-h6">{{ t('imLarkQrDialogTitle') }}</div>
        <div class="text-caption text-grey-7 q-mt-xs">{{ t('imLarkQrDialogDesc') }}</div>
      </q-card-section>

      <q-card-section class="q-pt-none column items-center">
        <div v-if="starting && !session" class="column items-center q-py-lg text-grey-7">
          <q-spinner-dots size="32px" color="primary" />
          <div class="q-mt-sm text-body2">{{ t('imLarkQrStarting') }}</div>
        </div>

        <q-banner v-if="error" dense rounded class="bg-negative text-white full-width q-mb-md">
          {{ error }}
        </q-banner>

        <q-banner
          v-else-if="isTerminalFailure"
          dense
          rounded
          class="bg-grey-3 text-grey-8 full-width q-mb-md"
        >
          {{ statusLabel }}
        </q-banner>

        <div v-if="showQr && qrDataUrl" class="column items-center q-gutter-sm">
          <div class="lark-qr-frame q-pa-md">
            <img :src="qrDataUrl" :alt="t('imLarkQrAlt')" width="200" height="200">
          </div>
          <div class="text-body2 text-grey-7 text-center">{{ statusLabel }}</div>
        </div>

        <div
          v-else-if="!starting && session && !isTerminalFailure && !error"
          class="column items-center q-py-md text-grey-7"
        >
          <q-icon name="qr_code_2" size="40px" class="q-mb-sm" />
          <div class="text-body2">{{ statusLabel || t('imLarkQrProcessing') }}</div>
          <q-spinner-dots size="24px" color="primary" class="q-mt-sm" />
        </div>
      </q-card-section>

      <q-card-actions align="right">
        <q-btn
          v-if="isTerminalFailure"
          flat
          color="primary"
          :label="t('imLarkQrRetry')"
          :loading="starting"
          @click="retry"
        />
        <q-btn flat :label="t('cancel')" @click="close" />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import QRCode from 'qrcode'
import {
  LARK_REGISTER_STATUS_LABELS,
  useLarkRegisterAppSession
} from 'src/pages/useLarkRegisterAppSession'
import type { LarkRegisterAppSession } from 'src/api/larkRegisterApp'

const props = defineProps<{
  open: boolean
  agentId: number | null
  appName?: string
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  completed: [session: LarkRegisterAppSession]
  failed: [message: string]
}>()

const { t } = useI18n()
const qrDataUrl = ref('')
const completedRef = ref(false)

const {
  session,
  starting,
  error,
  startSession,
  cancelSession,
  stopPoll
} = useLarkRegisterAppSession(() => props.agentId)

const status = computed(() => session.value?.status)
const statusLabel = computed(() => {
  if (!status.value) return starting.value ? t('imLarkQrConnecting') : ''
  return LARK_REGISTER_STATUS_LABELS[status.value] ?? session.value?.message ?? ''
})
const showQr = computed(() => status.value === 'qr_ready' || status.value === 'polling')
const isTerminalFailure = computed(() =>
  status.value === 'denied' ||
  status.value === 'expired' ||
  status.value === 'failed' ||
  status.value === 'cancelled'
)

watch(
  () => session.value?.qr_url,
  (url) => {
    if (!url) {
      qrDataUrl.value = ''
      return
    }
    void QRCode.toDataURL(url, { width: 200, margin: 1 }).then((data) => {
      qrDataUrl.value = data
    }).catch(() => {
      qrDataUrl.value = ''
    })
  }
)

watch(
  () => props.open,
  (isOpen) => {
    if (isOpen) {
      completedRef.value = false
      void startSession(onCompleted, onFailed, props.appName)
    } else {
      stopPoll()
    }
  }
)

function onCompleted (s: LarkRegisterAppSession) {
  completedRef.value = true
  emit('completed', s)
  emit('update:open', false)
}

function onFailed (msg: string) {
  emit('failed', msg)
}

function onOpenChange (val: boolean) {
  if (!val && !completedRef.value) {
    void cancelSession()
  }
  if (!val) completedRef.value = false
  emit('update:open', val)
}

function close () {
  onOpenChange(false)
}

function retry () {
  void startSession(onCompleted, onFailed, props.appName)
}
</script>

<style scoped>
.lark-qr-frame {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 12px;
  background: #fff;
}
</style>
