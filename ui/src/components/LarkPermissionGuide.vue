<template>
  <div class="lark-perm-guide">
    <q-btn
      flat
      dense
      no-caps
      color="primary"
      class="lark-perm-guide__toggle q-px-none"
      :icon="expanded ? 'expand_less' : 'expand_more'"
      :label="t('imLarkPermGuideTitle')"
      @click="expanded = !expanded"
    />

    <q-slide-transition>
      <div v-show="expanded" class="lark-perm-guide__panel q-mt-sm">
        <p class="text-caption text-grey-8 q-mb-md" style="line-height: 1.5">
          {{ t('imLarkPermGuideIntro') }}
          <a
            href="https://open.feishu.cn/app"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary"
          >open.feishu.cn</a>
          {{ t('imLarkPermGuideIntroIntl') }}
          <a
            href="https://open.larksuite.com/app"
            target="_blank"
            rel="noopener noreferrer"
            class="text-primary"
          >open.larksuite.com</a>
          {{ t('imLarkPermGuideIntroSuffix') }}
        </p>

        <q-expansion-item
          v-for="section in sections"
          :key="section.id"
          dense
          expand-separator
          header-class="lark-perm-guide__section-header"
          :default-opened="section.defaultOpen"
          class="lark-perm-guide__section q-mb-sm"
        >
          <template #header>
            <q-item-section>
              <q-item-label class="text-body2 text-weight-medium">{{ t(section.titleKey) }}</q-item-label>
              <q-item-label caption>{{ t(section.descKey) }}</q-item-label>
            </q-item-section>
            <q-item-section side>
              <q-badge color="grey-4" text-color="grey-9" :label="String(sectionScopeCount(section))" />
            </q-item-section>
          </template>

          <div class="q-pa-sm bg-grey-1">
            <div v-for="group in section.groups" :key="group.categoryKey" class="q-mb-md">
              <div class="text-caption text-weight-bold q-mb-xs">
                {{ t(group.categoryKey) }}
                <span class="text-grey-7 text-weight-regular"> · {{ t(group.subtitleKey) }}</span>
              </div>
              <div class="row q-gutter-xs">
                <code
                  v-for="scope in group.scopes"
                  :key="scope"
                  class="lark-perm-guide__scope-chip"
                >{{ scope }}</code>
              </div>
            </div>
            <q-btn
              outline
              dense
              no-caps
              size="sm"
              color="primary"
              icon="content_copy"
              :label="t('imLarkPermCopySection', { n: sectionScopeCount(section) })"
              @click="copyScopes(sectionScopes(section))"
            />
          </div>
        </q-expansion-item>

        <div class="row q-gutter-sm q-mt-sm">
          <q-btn
            outline
            dense
            no-caps
            color="primary"
            icon="content_copy"
            :label="t('imLarkPermCopyBasic')"
            @click="copyScopes(basicScopes)"
          />
          <q-btn
            outline
            dense
            no-caps
            color="primary"
            icon="content_copy"
            :label="t('imLarkPermCopyAll')"
            @click="copyScopes(allScopes)"
          />
        </div>

        <q-banner dense rounded class="bg-blue-1 text-blue-10 q-mt-md">
          <div class="text-caption" style="line-height: 1.55">
            <span class="text-weight-medium">{{ t('imLarkPermStepsLabel') }}</span>
            {{ t('imLarkPermSteps') }}
          </div>
        </q-banner>

        <q-banner dense rounded class="bg-amber-1 text-amber-10 q-mt-sm">
          <div class="text-caption" style="line-height: 1.55">
            {{ t('imLarkPermWsHint') }}
          </div>
        </q-banner>
      </div>
    </q-slide-transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuasar } from 'quasar'

type PermissionGroup = {
  categoryKey: string
  subtitleKey: string
  scopes: string[]
}

type PermissionSection = {
  id: string
  titleKey: string
  descKey: string
  defaultOpen?: boolean
  groups: PermissionGroup[]
}

const PERMISSION_SECTIONS: PermissionSection[] = [
  {
    id: 'basic',
    titleKey: 'imLarkPermBasicTitle',
    descKey: 'imLarkPermBasicDesc',
    defaultOpen: true,
    groups: [
      {
        categoryKey: 'imLarkPermCatMessage',
        subtitleKey: 'imLarkPermCatMessageSub',
        scopes: [
          'im:message',
          'im:message:send',
          'im:message.group_at_msg:readonly',
          'im:message:send_as_bot',
          'im:resource',
          'im:message.reaction:write'
        ]
      },
      {
        categoryKey: 'imLarkPermCatChat',
        subtitleKey: 'imLarkPermCatChatSub',
        scopes: ['im:chat', 'im:chat:readonly', 'im:chat.members:read']
      },
      {
        categoryKey: 'imLarkPermCatUser',
        subtitleKey: 'imLarkPermCatUserSub',
        scopes: ['contact:user.id:readonly']
      }
    ]
  },
  {
    id: 'docs',
    titleKey: 'imLarkPermDocsTitle',
    descKey: 'imLarkPermDocsDesc',
    groups: [
      {
        categoryKey: 'imLarkPermCatDocx',
        subtitleKey: 'imLarkPermCatDocxSub',
        scopes: ['docx:document', 'docx:document:readonly']
      },
      {
        categoryKey: 'imLarkPermCatSheets',
        subtitleKey: 'imLarkPermCatSheetsSub',
        scopes: ['sheets:spreadsheet', 'sheets:spreadsheet:readonly']
      },
      {
        categoryKey: 'imLarkPermCatBitable',
        subtitleKey: 'imLarkPermCatBitableSub',
        scopes: ['bitable:app', 'bitable:app:readonly']
      }
    ]
  },
  {
    id: 'drive',
    titleKey: 'imLarkPermDriveTitle',
    descKey: 'imLarkPermDriveDesc',
    groups: [
      {
        categoryKey: 'imLarkPermCatDrive',
        subtitleKey: 'imLarkPermCatDriveSub',
        scopes: ['drive:drive', 'drive:drive:readonly']
      },
      {
        categoryKey: 'imLarkPermCatWiki',
        subtitleKey: 'imLarkPermCatWikiSub',
        scopes: ['wiki:wiki', 'wiki:wiki:readonly']
      }
    ]
  }
]

const { t } = useI18n()
const $q = useQuasar()
const expanded = ref(false)
const sections = PERMISSION_SECTIONS

const basicScopes = computed(() =>
  PERMISSION_SECTIONS[0]?.groups.flatMap(g => g.scopes) ?? []
)

const allScopes = computed(() =>
  PERMISSION_SECTIONS.flatMap(s => s.groups.flatMap(g => g.scopes))
)

function sectionScopes (section: PermissionSection): string[] {
  return section.groups.flatMap(g => g.scopes)
}

function sectionScopeCount (section: PermissionSection): number {
  return sectionScopes(section).length
}

function buildScopesJson (scopes: string[]): string {
  return JSON.stringify({ scopes: { tenant: scopes, user: [] } }, null, 2)
}

async function copyScopes (scopes: string[]) {
  const text = buildScopesJson(scopes)
  try {
    await navigator.clipboard.writeText(text)
    $q.notify({ type: 'positive', message: t('imLarkPermCopied'), timeout: 2000 })
  } catch {
    $q.notify({ type: 'negative', message: t('imLarkPermCopyFailed'), timeout: 3000 })
  }
}
</script>

<style scoped>
.lark-perm-guide__panel {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  padding: 12px;
  background: rgba(0, 0, 0, 0.02);
}

.lark-perm-guide__section {
  border: 1px solid rgba(0, 0, 0, 0.08);
  border-radius: 8px;
  overflow: hidden;
}

.lark-perm-guide__scope-chip {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid rgba(0, 0, 0, 0.1);
  background: #fff;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  line-height: 1.6;
}
</style>
