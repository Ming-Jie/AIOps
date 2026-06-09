<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6">{{ t('knowledge') }}</div>
      <q-space />
      <q-btn flat round dense icon="refresh" @click="load" :loading="loading" class="q-mr-sm">
        <q-tooltip>{{ t('refresh') }}</q-tooltip>
      </q-btn>
      <q-btn color="primary" :label="t('kbCreate')" icon="add" @click="openCreate" unelevated rounded />
    </div>

    <div v-if="!loading && items.length > 0" class="row q-col-gutter-md q-mb-md">
      <div class="col-12 col-sm-4">
        <q-card flat bordered class="stat-card">
          <q-card-section>
            <div class="text-caption text-grey-7">{{ t('kbStatTotal') }}</div>
            <div class="text-h5 text-weight-medium">{{ formatKbCount(summary.kbCount) }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12 col-sm-4">
        <q-card flat bordered class="stat-card">
          <q-card-section>
            <div class="text-caption text-grey-7">{{ t('kbStatDocs') }}</div>
            <div class="text-h5 text-weight-medium">{{ formatKbCount(summary.docCount) }}</div>
          </q-card-section>
        </q-card>
      </div>
      <div class="col-12 col-sm-4">
        <q-card flat bordered class="stat-card">
          <q-card-section>
            <div class="text-caption text-grey-7">{{ t('kbStatPublic') }}</div>
            <div class="text-h5 text-weight-medium">{{ formatKbCount(summary.publicCount) }}</div>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <div v-if="items.length > 0" class="row items-center q-mb-md q-gutter-sm">
      <q-input
        v-model="searchQuery"
        outlined
        dense
        clearable
        class="col"
        style="max-width: 360px"
        :placeholder="t('kbSearchPlaceholder')"
      >
        <template #prepend><q-icon name="search" /></template>
      </q-input>
      <span v-if="searchQuery" class="text-caption text-grey-7">
        {{ t('kbSearchResult', { n: filteredItems.length }) }}
      </span>
    </div>

    <div v-if="!loading && items.length === 0" class="text-grey-6 q-pa-xl text-center">
      <q-icon name="menu_book" size="48px" class="q-mb-sm" />
      <div>{{ t('kbEmpty') }}</div>
    </div>

    <div v-else-if="!loading && filteredItems.length === 0" class="text-grey-6 q-pa-xl text-center">
      <q-icon name="search_off" size="48px" class="q-mb-sm" />
      <div>{{ t('kbSearchEmpty') }}</div>
    </div>

    <div class="row q-col-gutter-md">
      <div v-for="kb in filteredItems" :key="kb.id" class="col-12 col-sm-6 col-md-4">
        <q-card flat bordered class="cursor-pointer kb-card" @click="openDetail(kb)">
          <q-card-section>
            <div class="row items-center no-wrap">
              <q-icon name="menu_book" color="primary" size="28px" class="q-mr-sm" />
              <div class="col ellipsis text-subtitle1 text-weight-medium">{{ kb.name }}</div>
              <q-btn v-if="kb.can_manage" flat round dense icon="edit" color="primary" size="sm" @click.stop="openEdit(kb)">
                <q-tooltip>{{ t('edit') }}</q-tooltip>
              </q-btn>
              <q-btn
                v-if="kb.can_manage"
                flat
                round
                dense
                icon="delete"
                :color="(kb.doc_count ?? 0) > 0 ? 'grey-5' : 'negative'"
                size="sm"
                @click.stop="remove(kb)"
              >
                <q-tooltip>
                  {{ (kb.doc_count ?? 0) > 0 ? t('kbDeleteBlockedShort') : t('delete') }}
                </q-tooltip>
              </q-btn>
            </div>
            <div class="text-caption text-grey-7 q-mt-sm kb-desc">{{ kb.description || '—' }}</div>
            <div class="row items-center q-mt-sm q-gutter-xs">
              <q-badge :color="kb.visibility === 'public' ? 'blue' : 'grey'" :label="kb.visibility === 'public' ? t('kbPublic') : t('kbPrivate')" />
              <q-badge color="teal" :label="t('kbDocCount', { n: kb.doc_count })" />
              <q-badge v-if="!kb.is_owner" outline color="grey-7" :label="t('kbShared')" />
              <q-space />
              <span class="text-caption text-grey-5">#{{ kb.id }}</span>
            </div>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <q-dialog v-model="dialogOpen">
      <q-card style="min-width: 400px">
        <q-card-section class="row items-center bg-primary text-white">
          <div class="text-h6">{{ isEdit ? t('edit') : t('kbCreate') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <q-input v-model="form.name" :label="t('kbName')" outlined class="q-mb-md" autofocus />
          <q-input v-model="form.description" :label="t('kbDescription')" outlined type="textarea" class="q-mb-md" />
          <div class="text-caption text-grey-7 q-mb-xs">{{ t('kbVisibility') }}</div>
          <q-option-group
            v-model="form.visibility"
            :options="[
              { label: t('kbPublicOpt'), value: 'public' },
              { label: t('kbPrivateOpt'), value: 'private' }
            ]"
            color="primary"
            inline
          />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="save" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useKnowledgeBasePage } from './useKnowledgeBasePage'

defineOptions({ name: 'KnowledgeBasePage' })

const {
  t, loading, items, filteredItems, searchQuery, summary, formatKbCount,
  dialogOpen, isEdit, form, load, openCreate, openEdit, save, remove, openDetail
} = useKnowledgeBasePage()

onMounted(load)
</script>

<style scoped>
.kb-card {
  transition: box-shadow 0.2s;
}
.kb-card:hover {
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.12);
}
.kb-desc {
  min-height: 2.4em;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.stat-card {
  min-height: 88px;
}
</style>
