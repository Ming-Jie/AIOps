<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h6 text-text2">{{ t('teams') }}</div>
      <q-space />
      <q-btn color="primary" :label="t('createTeam')" icon="add" @click="openDialog()" class="q-mr-sm" unelevated rounded />
      <q-btn flat icon="refresh" round dense @click="loadTeams" :loading="loading" />
    </div>

    <q-banner v-if="errorMsg" class="bg-negative text-white q-mb-md" dense>{{ errorMsg }}</q-banner>

    <!-- Mode Info Banner -->
    <q-banner class="bg-info text-white q-mb-md rounded-borders" dense v-if="teams.length > 0">
      <template #avatar><q-icon name="info" /></template>
      {{ t('teamModeInfo') }}
      <template #action>
        <q-btn flat dense :label="t('teamModeInfoTitle')" @click="modeInfoDialog = true" />
      </template>
    </q-banner>

    <div class="row q-col-gutter-md">
      <div v-for="team in teams" :key="team.id" class="col-12 col-sm-6 col-md-4">
        <q-card class="team-card cursor-pointer" @click="openTeamChat(team)">
          <q-card-section>
            <div class="row items-center no-wrap">
              <q-icon :name="modeIcon(team.mode)" size="md" color="primary" class="q-mr-sm" />
              <div class="text-subtitle1 text-weight-medium ellipsis">{{ team.name }}</div>
              <q-space />
              <q-chip dense :color="team.is_active ? 'positive' : 'grey'" text-color="white" size="sm">
                {{ team.is_active ? t('active') : t('inactive') }}
              </q-chip>
            </div>
            <div class="text-caption text-grey q-mt-xs">{{ modeLabel(team.mode) }}</div>
            <div v-if="team.description" class="text-body2 q-mt-sm text-grey-7 ellipsis-2-lines">{{ team.description }}</div>
          </q-card-section>

          <q-card-section class="q-pt-none">
            <div class="text-caption text-grey">{{ t('teamMembers') }} ({{ team.members?.length || 0 }})</div>
            <div class="row q-mt-xs q-gutter-xs">
              <q-chip v-for="m in team.members?.slice(0, 5)" :key="m.id" dense size="sm" color="secondary" text-color="white">
                {{ m.agent_name || `Agent(${m.agent_id})` }}
              </q-chip>
              <q-chip v-if="team.members && team.members.length > 5" dense size="sm">+{{ team.members.length - 5 }}</q-chip>
            </div>
          </q-card-section>

          <q-separator />

          <q-card-actions class="q-pa-sm">
            <q-btn
              flat dense color="primary" :label="t('teamStartConversation')" icon="chat"
              @click.stop="startConversation(team)" size="sm"
            />
            <q-space />
            <q-btn flat dense round icon="edit" @click.stop="openDialog(team)" />
            <q-btn flat dense round icon="delete" color="negative" @click.stop="confirmDelete(team)" />
          </q-card-actions>
        </q-card>
      </div>

      <div v-if="!loading && teams.length === 0" class="col-12">
        <q-banner class="bg-grey-2 rounded-borders">
          <template #avatar><q-icon name="groups" color="grey" /></template>
          {{ t('teamNoTeams') }}
        </q-banner>
      </div>
    </div>

    <!-- Mode Info Dialog -->
    <q-dialog v-model="modeInfoDialog">
      <q-card style="min-width: 400px">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ t('teamModeInfoTitle') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>
        <q-card-section>
          <q-list dense>
            <q-item v-for="mode in modeDescriptions" :key="mode.value">
              <q-item-section avatar><q-icon :name="mode.icon" color="primary" /></q-item-section>
              <q-item-section>
                <q-item-label>{{ mode.label }}</q-item-label>
                <q-item-label caption>{{ mode.desc }}</q-item-label>
              </q-item-section>
            </q-item>
          </q-list>
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat :label="t('ok')" v-close-popup />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Team Editor Dialog -->
    <q-dialog v-model="dialogOpen">
      <q-card style="width: min(92vw, 640px); max-width: 92vw;">
        <q-card-section class="row items-center">
          <div class="text-h6">{{ editingId ? t('editTeam') : t('createTeam') }}</div>
          <q-space />
          <q-btn icon="close" flat round dense v-close-popup />
        </q-card-section>

        <q-card-section class="q-pt-none scroll" style="max-height: 70vh;">
          <div class="row q-col-gutter-md">
            <div class="col-8">
              <q-input v-model="form.name" :label="t('teamName')" outlined dense :rules="[v => !!v || t('teamNameRequired')]" />
            </div>
            <div class="col-4">
              <q-select v-model="form.mode" :label="t('teamMode')" :options="modeOptions" outlined dense emit-value map-options />
            </div>
          </div>

          <q-input v-model="form.description" :label="t('description')" outlined dense type="textarea" autogrow class="q-mt-md" />

          <div class="row q-col-gutter-md q-mt-md">
            <div class="col-6">
              <q-select
                v-model="form.coordinator_agent_id" :label="t('teamCoordinator')" :options="agentOptions"
                clearable outlined dense emit-value map-options
              />
            </div>
            <div class="col-4">
              <q-input v-model="form.max_rounds" :label="t('teamMaxRounds')" outlined dense type="number" min="1" max="50" />
            </div>
          </div>

          <!-- Mode-specific config -->
          <template v-if="form.mode === 'debate'">
            <div class="row q-col-gutter-md q-mt-md">
              <div class="col-4">
                <q-input v-model="debateRounds" :label="t('teamDebateRounds')" outlined dense type="number" min="1" max="10" />
              </div>
            </div>
          </template>

          <!-- Agent selection -->
          <div class="q-mt-lg">
            <div class="text-subtitle2 q-mb-sm">{{ t('teamMembers') }}</div>
            <q-select
              v-model="form.agent_ids" :label="t('teamSelectMembers')" :options="agentOptions"
              multiple outlined dense chips use-chips emit-value map-options
            />

            <template v-if="form.mode === 'debate' && form.agent_ids.length > 0">
              <div class="q-mt-md">
                <div class="text-subtitle2 q-mb-sm">{{ t('teamConfigStances') }}</div>
                <div class="text-caption text-grey-7 q-mb-sm">{{ t('teamStanceHint') }}</div>
                <div v-for="aid in form.agent_ids" :key="aid" class="row items-center q-mb-sm q-gutter-sm">
                  <div class="col-3 text-caption text-weight-medium">{{ getAgentName(aid) }}</div>
                  <div class="col-8">
                    <q-input v-model="stanceMap[aid]" outlined dense :placeholder="t('teamStanceHint')" />
                  </div>
                </div>
              </div>
            </template>
          </div>
        </q-card-section>

        <q-card-actions align="right">
          <q-btn flat :label="t('cancel')" v-close-popup />
          <q-btn color="primary" :label="t('save')" @click="saveTeam" :loading="saving" unelevated />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup lang="ts">
import { useTeamsPage } from './useTeamsPage'

defineOptions({ name: 'TeamsPage' })

const {
  t, loading, saving, teams, errorMsg, dialogOpen, editingId, form,
  modeInfoDialog, modeOptions, agentOptions,
  debateRounds, stanceMap, modeDescriptions,
  loadTeams, openDialog, saveTeam, confirmDelete,
  openTeamChat, startConversation, modeLabel, modeIcon, getAgentName
} = useTeamsPage()
</script>

<style scoped>
.team-card {
  transition: box-shadow 0.2s;
}
.team-card:hover {
  box-shadow: 0 2px 12px rgba(0,0,0,0.12);
}
.ellipsis-2-lines {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
