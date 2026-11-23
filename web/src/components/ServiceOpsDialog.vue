<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import { ApiError } from '@/api/client'
import { servicesApi } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiTag from '@/components/ui/UiTag.vue'
import type {
  ServiceInstance,
  TargetAdminAudit,
  TargetAdminPlugin,
} from '@/types'

/**
 * 远程运营对话框：插件上下架、全站公告、审计日志。
 * 与 ServiceAdminDialog（用户管理）并列，均为经 adapter 代理目标服务的站长 API。
 */
const props = defineProps<{
  visible?: boolean
  instance?: ServiceInstance | null
}>()

const emit = defineEmits<{ 'update:visible': [value: boolean] }>()
const { t } = useI18n()

const tab = ref<'plugins' | 'announce' | 'audits'>('plugins')
const loading = ref(false)
const plugins = ref<TargetAdminPlugin[]>([])
const audits = ref<TargetAdminAudit[]>([])
const announceContent = ref('')
const announcing = ref(false)

const pluginPage = ref(1)
const auditPage = ref(1)
const pageSize = 20

watch(
  () => props.visible,
  (v) => {
    if (v) {
      tab.value = 'plugins'
      pluginPage.value = 1
      auditPage.value = 1
      void reload()
    }
  },
)

async function reload() {
  if (!props.instance) return
  loading.value = true
  try {
    const [p, a] = await Promise.all([
      servicesApi.adminPlugins(props.instance.id, pluginPage.value, pageSize),
      servicesApi.adminAudits(props.instance.id, auditPage.value, pageSize),
    ])
    plugins.value = p.plugins ?? []
    audits.value = a.audits ?? []
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    loading.value = false
  }
}

async function togglePlugin(p: TargetAdminPlugin) {
  if (!props.instance || loading.value) return
  loading.value = true
  try {
    const r = await servicesApi.adminSetPlugin(props.instance.id, p.name, !p.globalDisabled)
    MessagePlugin.success(t('ops.pluginDone', { name: p.name, count: r.affected }))
    await reload()
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    loading.value = false
  }
}

async function sendAnnounce() {
  if (!props.instance || announcing.value) return
  const content = announceContent.value.trim()
  if (!content) {
    MessagePlugin.warning(t('ops.announceRequired'))
    return
  }
  announcing.value = true
  try {
    const r = await servicesApi.adminAnnounce(props.instance.id, content)
    MessagePlugin.success(t('ops.announceDone', { count: r.recipients }))
    announceContent.value = ''
    await reload()
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    announcing.value = false
  }
}

function close() {
  emit('update:visible', false)
}
</script>

<template>
  <UiDialog
    :visible="props.visible ?? false"
    :header="t('ops.title')"
    width="760px"
    :footer="false"
    @update:visible="close"
  >
    <div class="ops">
      <div class="ops__tabs">
        <UiButton variant="text" size="small" @click="tab = 'plugins'">{{ t('ops.plugins') }}</UiButton>
        <UiButton variant="text" size="small" @click="tab = 'announce'">{{ t('ops.announce') }}</UiButton>
        <UiButton variant="text" size="small" @click="tab = 'audits'">{{ t('ops.audits') }}</UiButton>
        <UiButton variant="text" size="small" :loading="loading" @click="reload">{{ t('common.refresh') }}</UiButton>
      </div>

      <div v-if="tab === 'plugins'" class="ops__panel">
        <p v-if="!plugins.length" class="ops__empty">{{ t('ops.empty') }}</p>
        <div v-for="p in plugins" :key="p.name" class="ops__row">
          <div>
            <strong>{{ p.name }}</strong>
            <span class="ops__meta">v{{ p.version }} · {{ p.installCount }} {{ t('ops.installs') }}</span>
          </div>
          <UiTag :theme="p.globalDisabled ? 'default' : 'success'">
            {{ p.globalDisabled ? t('ops.disabled') : t('ops.enabled') }}
          </UiTag>
          <UiButton variant="outline" size="small" @click="togglePlugin(p)">
            {{ p.globalDisabled ? t('ops.restore') : t('ops.disable') }}
          </UiButton>
        </div>
      </div>

      <div v-else-if="tab === 'announce'" class="ops__panel">
        <p class="ops__hint">{{ t('ops.announceHint') }}</p>
        <textarea
          v-model="announceContent"
          class="ops__textarea"
          rows="5"
          :placeholder="t('ops.announcePlaceholder')"
        />
        <div class="ops__actions">
          <UiButton theme="primary" :loading="announcing" @click="sendAnnounce">
            {{ t('ops.send') }}
          </UiButton>
        </div>
      </div>

      <div v-else class="ops__panel">
        <p v-if="!audits.length" class="ops__empty">{{ t('ops.empty') }}</p>
        <div v-for="a in audits" :key="a.id" class="ops__row">
          <div>
            <strong>{{ a.action }}</strong>
            <span class="ops__meta">
              {{ a.targetType }} {{ a.targetName }} · {{ a.ip }} · {{ a.createdAt }}
            </span>
          </div>
          <span class="ops__detail">{{ a.detail }}</span>
        </div>
      </div>
    </div>
  </UiDialog>
</template>

<style scoped>
.ops__tabs {
  display: flex;
  gap: 8px;
  align-items: center;
  margin-bottom: 12px;
}

.ops__panel {
  min-height: 220px;
}

.ops__row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--mc-hairline);
}

.ops__meta {
  display: block;
  font-size: 12px;
  color: var(--mc-text-faint);
}

.ops__detail {
  max-width: 280px;
  font-size: 12px;
  color: var(--mc-text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ops__hint,
.ops__empty {
  font-size: 13px;
  color: var(--mc-text-faint);
}

.ops__textarea {
  width: 100%;
  box-sizing: border-box;
  padding: 10px;
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-sm);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-family: inherit;
  font-size: 13px;
  resize: vertical;
}

.ops__actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 12px;
}
</style>
