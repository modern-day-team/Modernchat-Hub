<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import { ApiError } from '@/api/client'
import { servicesApi } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { useAuthStore } from '@/stores/auth'
import { useInstancesStore } from '@/stores/instances'
import type { ServiceInstance } from '@/types'

const { t } = useI18n()
const instances = useInstancesStore()
const auth = useAuthStore()
const loaded = ref(false)
const loadError = ref('')
const checkingAll = ref(false)

async function load() {
  loadError.value = ''
  try {
    await instances.fetchAll()
  } catch (e) {
    // 列表加载失败给页面级错误态 + 重试入口，不能只靠 toast
    loadError.value = e instanceof ApiError ? e.message : t('svc.failed')
  } finally {
    loaded.value = true
  }
}

onMounted(load)

const counts = computed(() => {
  const total = instances.list.length
  const ok = instances.list.filter((i) => i.lastStatus === 'ok').length
  const fail = instances.list.filter((i) => i.lastStatus === 'fail').length
  return { total, ok, fail, unknown: total - ok - fail }
})

/** 一键批量探测（后端有界并发），完成即刷新统计 */
async function checkAll() {
  if (checkingAll.value) return
  checkingAll.value = true
  try {
    const summary = await instances.checkAll()
    MessagePlugin.success(t('dash.checkAllDone', { ok: summary.ok, fail: summary.fail }))
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    checkingAll.value = false
  }
}

function statusTheme(s: ServiceInstance['lastStatus']): 'success' | 'danger' | 'default' {
  return s === 'ok' ? 'success' : s === 'fail' ? 'danger' : 'default'
}

function statusText(s: ServiceInstance['lastStatus']) {
  return s === 'ok' ? t('svc.statusOk') : s === 'fail' ? t('svc.statusFail') : t('svc.statusUnknown')
}

function fmtTime(iso?: string) {
  return iso ? new Date(iso).toLocaleString() : t('dash.none')
}

// ── OAuth 客户端注册（统一登录中心；仅管理员可见）──
const oauthClientID = ref('')
const oauthRedirect = ref('')
const oauthScopes = ref('profile')
const oauthSecret = ref('')
const registeringOAuth = ref(false)

async function registerOAuthClient() {
  if (registeringOAuth.value) return
  if (!oauthClientID.value.trim() || !oauthRedirect.value.trim()) {
    MessagePlugin.warning(t('oauth.required'))
    return
  }
  registeringOAuth.value = true
  try {
    const r = await servicesApi.registerOAuthClient({
      clientId: oauthClientID.value.trim(),
      redirectUri: oauthRedirect.value.trim(),
      scopes: oauthScopes.value.trim() || undefined,
    })
    oauthSecret.value = r.clientSecret
    MessagePlugin.success(t('oauth.created'))
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    registeringOAuth.value = false
  }
}
</script>

<template>
  <div class="mc-page mc-page--wide">
    <div class="mc-page-head">
      <h1 class="mc-page-head__title">{{ t('dash.title') }}</h1>
      <div class="mc-page-head__actions">
        <UiButton variant="outline" size="small" :loading="checkingAll" :disabled="loaded && instances.list.length === 0" @click="checkAll">
          {{ checkingAll ? t('dash.checkingAll') : t('dash.checkAll') }}
        </UiButton>
      </div>
    </div>

    <!-- 状态统计卡：<480 单列，≥480 两列，≥768 四列 -->
    <div class="mc-grid dash__stats">
      <div class="mc-stat mc-col-3">
        <div class="mc-stat__value">{{ counts.total }}</div>
        <div class="mc-stat__label">{{ t('dash.total') }}</div>
      </div>
      <div class="mc-stat mc-stat--ok mc-col-3">
        <div class="mc-stat__value">{{ counts.ok }}</div>
        <div class="mc-stat__label">{{ t('dash.healthy') }}</div>
      </div>
      <div class="mc-stat mc-stat--danger mc-col-3">
        <div class="mc-stat__value">{{ counts.fail }}</div>
        <div class="mc-stat__label">{{ t('dash.abnormal') }}</div>
      </div>
      <div class="mc-stat mc-col-3">
        <div class="mc-stat__value">{{ counts.unknown }}</div>
        <div class="mc-stat__label">{{ t('dash.unknown') }}</div>
      </div>
    </div>

    <!-- 服务状态简表 -->
    <div class="mc-panel dash__panel">
      <!-- 加载失败：页面级错误态 + 重试 -->
      <div v-if="loadError" class="dash__empty" role="alert">
        <p>{{ loadError }}</p>
        <UiButton variant="outline" size="small" class="dash__go" @click="load">{{ t('dash.retry') }}</UiButton>
      </div>
      <!-- 首次加载中 -->
      <div v-else-if="!loaded" class="dash__empty">
        <p>{{ t('dash.loading') }}</p>
      </div>
      <div v-else-if="instances.list.length === 0" class="dash__empty">
        <p>{{ t('dash.noServices') }}</p>
        <router-link to="/services" class="dash__go">{{ t('dash.goAdd') }}</router-link>
      </div>
      <!-- 窄屏表格横向滚动，不撑破面板 -->
      <div v-else class="dash__scroll">
        <table class="dash__table">
          <thead>
            <tr>
              <th>{{ t('dash.name') }}</th>
              <th>{{ t('dash.status') }}</th>
              <th>{{ t('dash.version') }}</th>
              <th>{{ t('dash.lastCheck') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="inst in instances.list" :key="inst.id">
              <td class="dash__name">{{ inst.name }}</td>
              <td><UiTag :theme="statusTheme(inst.lastStatus)">{{ statusText(inst.lastStatus) }}</UiTag></td>
              <td>{{ inst.lastVersion ?? '—' }}</td>
              <td class="dash__time">{{ fmtTime(inst.lastCheckAt) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 统一登录中心：OAuth 客户端注册（仅管理员） -->
    <div v-if="auth.user?.role === 'admin'" class="mc-panel oauth">
      <h2 class="mc-panel__title">{{ t('oauth.title') }}</h2>
      <p class="mc-panel__desc oauth__hint">{{ t('oauth.hint') }}</p>
      <!-- ≥1024：三个字段 + 按钮各占 3 列；<1024 逐个堆叠 -->
      <div class="mc-grid oauth__form">
        <label class="mc-col-3 oauth__field">
          <span class="ui-label">client_id</span>
          <UiInput v-model="oauthClientID" :placeholder="t('oauth.clientIdPlaceholder')" />
        </label>
        <label class="mc-col-3 oauth__field">
          <span class="ui-label">redirect_uri</span>
          <UiInput v-model="oauthRedirect" :placeholder="t('oauth.redirectPlaceholder')" />
        </label>
        <label class="mc-col-3 oauth__field">
          <span class="ui-label">scope</span>
          <UiInput v-model="oauthScopes" placeholder="profile" />
        </label>
        <div class="mc-col-3 oauth__action">
          <UiButton theme="primary" block :loading="registeringOAuth" @click="registerOAuthClient">
            {{ t('oauth.register') }}
          </UiButton>
        </div>
      </div>
      <div v-if="oauthSecret" class="oauth__secret">
        <p class="oauth__hint oauth__hint--tight">{{ t('oauth.secretHint') }}</p>
        <code>{{ oauthSecret }}</code>
      </div>
    </div>
  </div>
</template>

<style scoped>
/* 统计卡：hover 轻微提层（描边转品牌色淡档 + 阴影升一级） */
.dash__stats .mc-stat {
  transition:
    border-color var(--mc-dur-instant) var(--mc-ease-out),
    box-shadow var(--mc-dur-instant) var(--mc-ease-out);
}

.dash__stats .mc-stat:hover {
  border-color: color-mix(in srgb, var(--td-brand-color) 40%, transparent);
  box-shadow: var(--mc-elev-2);
}

/* 表格面板：比默认面板更紧的上下内边距，左右与面板一致 */
.dash__panel {
  padding: var(--mc-sp-2) var(--mc-sp-5);
}

.dash__scroll {
  overflow-x: auto;
}

.dash__table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--mc-fs-body);
}

.dash__table th {
  text-align: left;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  padding: var(--mc-sp-3) var(--mc-sp-2);
  border-bottom: 1px solid var(--mc-hairline);
}

.dash__table td {
  padding: var(--mc-sp-3) var(--mc-sp-2);
  border-bottom: 1px solid var(--mc-hairline);
}

.dash__table th,
.dash__table td {
  white-space: nowrap;
}

.dash__table tbody tr {
  transition: background-color var(--mc-dur-instant) var(--mc-ease-out);
}

.dash__table tbody tr:hover {
  background: var(--td-bg-color-container-hover);
}

.dash__table tr:last-child td {
  border-bottom: none;
}

.dash__name {
  font-weight: 500;
}

.dash__time {
  color: var(--td-text-color-secondary);
  font-size: var(--mc-fs-sm);
}

.dash__empty {
  padding: var(--mc-sp-8) var(--mc-sp-2);
  text-align: center;
  color: var(--td-text-color-secondary);
}

.dash__empty p {
  margin: 0;
}

.dash__go {
  display: inline-block;
  margin-top: var(--mc-sp-2);
  font-size: var(--mc-fs-sm);
}

/* OAuth 注册表单 */
.oauth__hint {
  max-width: var(--mc-container-compact);
  margin-bottom: var(--mc-sp-4);
}

.oauth__hint--tight {
  margin-bottom: var(--mc-sp-1);
}

/* 移动优先：<1024 全部占满一行（覆盖 .mc-col-3 在 ≥480 / ≥768 的并排规则） */
.oauth__form > * {
  grid-column: 1 / -1;
}

@media (min-width: 1024px) {
  .oauth__form > * {
    grid-column: span 3;
  }
}

.oauth__field {
  display: flex;
  flex-direction: column;
  gap: var(--mc-sp-1);
}

.oauth__action {
  padding-top: calc(var(--mc-fs-body) * var(--mc-lh-normal) + var(--mc-sp-1));
}

.oauth__secret {
  margin-top: var(--mc-sp-4);
  padding: var(--mc-sp-3);
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-md);
}

.oauth__secret code {
  font-size: var(--mc-fs-sm);
  word-break: break-all;
}
</style>
