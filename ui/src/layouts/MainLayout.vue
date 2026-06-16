<template>
  <q-layout view="hHh Lpr lFf">
    <q-header elevated class="bg-ai-gradient text-white header-main">
      <q-toolbar>
        <q-btn flat dense round icon="menu" aria-label="Menu" @click="leftDrawer = !leftDrawer" />
        <q-toolbar-title class="row items-center q-gutter-x-sm">
          <q-icon name="auto_awesome" size="sm" class="header-sparkle" />
          <span>{{ t('appTitle') }}</span>
        </q-toolbar-title>
        <q-space />
        <q-btn-dropdown
          flat
          dense
          round
          text-color="white"
          icon="language"
          :aria-label="t('language')"
        >
          <q-list dense style="min-width: 180px">
            <q-item
              clickable
              v-close-popup
              :active="locale === 'en-US'"
              active-class="text-primary"
              @click="setLocale('en-US')"
            >
              <q-item-section>{{ t('langLocaleEn') }}</q-item-section>
              <q-item-section v-if="locale === 'en-US'" side>
                <q-icon name="check" size="xs" color="primary" />
              </q-item-section>
            </q-item>
            <q-item
              clickable
              v-close-popup
              :active="locale === 'zh-CN'"
              active-class="text-primary"
              @click="setLocale('zh-CN')"
            >
              <q-item-section>{{ t('langLocaleZh') }}</q-item-section>
              <q-item-section v-if="locale === 'zh-CN'" side>
                <q-icon name="check" size="xs" color="primary" />
              </q-item-section>
            </q-item>
          </q-list>
        </q-btn-dropdown>
      </q-toolbar>
    </q-header>

    <q-drawer v-model="leftDrawer" show-if-above bordered class="main-layout-drawer bg-white" :width="240">
      <div class="column no-wrap fit">
        <q-scroll-area class="main-layout-drawer-scroll col">
          <q-list padding class="q-mt-xs">
            <q-item clickable v-ripple :to="{ name: 'dashboard' }" exact active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="dashboard" />
              </q-item-section>
              <q-item-section>{{ t('dashboard') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'agents' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="smart_toy" />
              </q-item-section>
              <q-item-section>{{ t('agents') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'chat' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="chat" />
              </q-item-section>
              <q-item-section>{{ t('chat') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'channels' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="notifications" />
              </q-item-section>
              <q-item-section>{{ t('channels') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'approvals' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="approval" />
              </q-item-section>
              <q-item-section>{{ t('approvals') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'schedules' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="schedule" />
              </q-item-section>
              <q-item-section>{{ t('schedules') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'teams' }" :active="isTeamsNavActive" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="groups" />
              </q-item-section>
              <q-item-section>{{ t('teams') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'knowledge' }" :active="isKnowledgeNavActive" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="menu_book" />
              </q-item-section>
              <q-item-section>{{ t('knowledge') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'skills' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="build" />
              </q-item-section>
              <q-item-section>{{ t('skills') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'mcp' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="hub" />
              </q-item-section>
              <q-item-section>{{ t('mcp') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'workflows' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="account_tree" />
              </q-item-section>
              <q-item-section>{{ t('workflows') }}</q-item-section>
            </q-item>
            <q-item clickable v-ripple :to="{ name: 'bots' }" active-class="drawer-item-active">
              <q-item-section avatar>
                <q-icon name="smart_toy" />
              </q-item-section>
              <q-item-section>{{ t('botTitle') }}</q-item-section>
            </q-item>
            <template v-if="meLoaded && isAdmin">
              <q-separator class="q-my-sm" />
              <q-item-label header class="text-uppercase text-caption text-grey-5 q-px-md q-mt-sm q-mb-xs" style="font-weight: 600; letter-spacing: 0.06em;">{{ t('admin') }}</q-item-label>
              <q-item clickable v-ripple :to="{ name: 'models' }" active-class="drawer-item-active">
                <q-item-section avatar>
                  <q-icon name="model_training" />
                </q-item-section>
                <q-item-section>{{ t('models') }}</q-item-section>
              </q-item>
              <!-- q-item clickable v-ripple :to="{ name: 'guardrails' }">
                  <q-item-section avatar>
                    <q-icon name="shield" />
                  </q-item-section>
                  <q-item-section>{{ t('guardrails') }}</q-item-section>
                </q-item -->
              <q-item clickable v-ripple :to="{ name: 'eval' }" active-class="drawer-item-active">
                <q-item-section avatar>
                  <q-icon name="fact_check" />
                </q-item-section>
                <q-item-section>{{ t('eval') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'audit-logs' }" active-class="drawer-item-active">
                <q-item-section avatar>
                  <q-icon name="receipt_long" />
                </q-item-section>
                <q-item-section>{{ t('auditLogs') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'roles' }" active-class="drawer-item-active">
                <q-item-section avatar>
                  <q-icon name="supervisor_account" />
                </q-item-section>
                <q-item-section>{{ t('roles') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'users' }" active-class="drawer-item-active">
                <q-item-section avatar>
                  <q-icon name="people" />
                </q-item-section>
                <q-item-section>{{ t('users') }}</q-item-section>
              </q-item>
              <q-item clickable v-ripple :to="{ name: 'usage' }" active-class="drawer-item-active">
                <q-item-section avatar>
                  <q-icon name="insights" />
                </q-item-section>
                <q-item-section>{{ t('usageStats') }}</q-item-section>
              </q-item>
            </template>
          </q-list>
        </q-scroll-area>

        <q-separator />
        <div class="main-layout-drawer-user q-pa-md">
          <div class="row items-center no-wrap q-gutter-sm">
            <q-avatar color="primary" text-color="white" size="40px" class="cursor-pointer" @click="goProfile">
              <img v-if="avatarUrl" :src="avatarUrl" alt="">
              <q-icon v-else name="person" />
            </q-avatar>
            <div class="col main-layout-drawer-user-text cursor-pointer" @click="goProfile">
              <div class="text-body2 text-weight-medium ellipsis">{{ userLabel || '—' }}</div>
              <div class="text-caption text-grey-7">{{ t('profile') }}</div>
            </div>
            <q-btn flat dense round icon="logout" :aria-label="t('logout')" @click="onLogout" />
          </div>
        </div>
      </div>
    </q-drawer>

    <q-page-container class="main-layout-page-container">
      <router-view v-slot="{ Component }">
        <transition name="page-fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </q-page-container>
  </q-layout>
</template>

<script setup lang="ts">
import { useMainLayout } from './mainLayout'

defineOptions({
  name: 'MainLayout'
})

const {
  t, locale, setLocale, leftDrawer, userLabel, avatarUrl, meLoaded, isAdmin,
  isTeamsNavActive, isKnowledgeNavActive,
  onLogout, goProfile
} = useMainLayout()
</script>

<style scoped>
.header-main {
  backdrop-filter: blur(8px);
  box-shadow: 0 2px 12px rgba(99, 102, 241, 0.2) !important;
}
.header-sparkle {
  animation: sparkle-pulse 2s ease-in-out infinite;
}
@keyframes sparkle-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.7; transform: scale(0.9); }
}
.main-layout-drawer {
  border-right: 1px solid var(--color-border-light, rgba(0, 0, 0, 0.06));
}
.main-layout-drawer-scroll {
  flex: 1 1 0;
  min-height: 0;
  /* 让 q-list 内部项目有更舒服的间距 */
  :deep(.q-item) {
    border-radius: var(--radius-md, 10px);
    margin: 2px 8px;
    padding: 10px 12px !important;
    transition: background-color 0.15s ease;
  }
  :deep(.q-item__section--avatar) {
    min-width: 36px;
  }
  :deep(.q-item:hover) {
    background: rgba(0, 0, 0, 0.03);
  }
  :deep(.q-item.drawer-item-active) {
    background: rgba(99, 102, 241, 0.08) !important;
    color: var(--ai-primary) !important;
    font-weight: 600;
  }
  :deep(.q-item.drawer-item-active .q-icon) {
    color: var(--ai-primary) !important;
  }
}
.main-layout-drawer-user-text {
  min-width: 0;
}
</style>
