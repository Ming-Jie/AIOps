<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('models') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createModel')" icon="add" @click="openDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense @click="load" :loading="loading" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <q-table
      flat
      bordered
      class="q-table--soft radius-md"
      :rows="rows"
      :columns="columns"
      row-key="id"
      :loading="loading"
      :rows-per-page-options="[10, 20, 50]"
      :no-data-label="t('noData')"
    >
      <template #body-cell-provider="props">
        <q-td :props="props">
          <q-badge :color="providerColor(props.row.provider)" :label="props.row.provider" />
        </q-td>
      </template>
      <template #body-cell-is_active="props">
        <q-td :props="props">
          <q-badge :color="props.row.is_active !== false ? 'positive' : 'grey'" :label="props.row.is_active !== false ? t('roleEnabled') : t('roleDisabled')" />
        </q-td>
      </template>
      <template #body-cell-actions="props">
        <q-td :props="props">
          <q-btn dense flat color="secondary" :label="t('edit')" @click="openDialog(props.row)" />
          <q-btn dense flat color="negative" :label="t('delete')" @click="confirmDelete(props.row)" />
        </q-td>
      </template>
    </q-table>

    <q-dialog v-model="dialogOpen" transition-show="scale" transition-hide="scale">
      <q-card class="card-soft card-soft--bordered" style="width: min(92vw, 520px); max-width: 92vw; border-radius: var(--radius-xl, 18px) !important;">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ editingId ? t('editModel') : t('createModel') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none">
          <q-input v-model="form.name" :label="t('modelName')" outlined dense :rules="[v => !!v]" class="q-mb-md" />
          <div class="row q-col-gutter-md q-mb-md">
            <div class="col-6">
              <q-select
                v-model="form.purpose"
                :options="purposeOptions"
                :label="t('modelPurpose')"
                outlined
                dense
                emit-value
                map-options
              />
            </div>
            <div class="col-6">
              <q-select
                v-model="form.provider"
                :options="providerOptions"
                :label="t('modelProvider')"
                outlined
                dense
                emit-value
                map-options
              />
            </div>
          </div>
          <div class="row q-col-gutter-md q-mb-md">
            <div class="col-6">
              <q-input v-model="form.model" :label="t('modelModel')" outlined dense :rules="[v => !!v]" />
            </div>
            <div class="col-6">
              <q-input v-model="form.base_url" :label="t('modelBaseUrl')" outlined dense />
            </div>
          </div>
          <q-input
            v-model="form.api_key"
            :label="t('modelApiKey')"
            outlined
            dense
            :type="showApiKey ? 'text' : 'password'"
            class="q-mb-md"
          >
            <template #append>
              <q-btn flat dense round :icon="showApiKey ? 'visibility_off' : 'visibility'" @click="showApiKey = !showApiKey" />
            </template>
          </q-input>
          <q-checkbox v-model="form.is_active" :label="t('isActive')" />
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveModel" :loading="saving" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { useModelsPage } from './useModelsPage'

defineOptions({ name: 'ModelsPage' })

const {
  t,
  loading,
  saving,
  rows,
  errorMsg,
  columns,
  load,
  dialogOpen,
  editingId,
  form,
  showApiKey,
  openDialog,
  saveModel,
  confirmDelete,
  providerOptions,
  providerColor,
  purposeOptions
} = useModelsPage()
</script>
