<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { setLang } from '@/i18n'
import { useAuthStore } from '@/stores/auth'
import { useInstancesStore } from '@/stores/instances'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const instances = useInstancesStore()

// 登录/注册页不套面板壳，居中卡片呈现
const bare = computed(() => route.name === 'login' || route.name === 'register')
const pageTitle = computed(() => (route.name === 'services' ? t('nav.services') : t('nav.overview')))

onMounted(() => {
  // 刷新后补拉当前用户（401 已由 client 统一处理）
  void auth.fetchMe()
  // 侧栏服务数需要在任意页面都准确；fetchAll 幂等，页面自身再拉也不会冲突
  if (!instances.loading && instances.list.length === 0) void instances.fetchAll()
})

function toggleLang() {
  setLang(locale.value === 'zh-CN' ? 'en-US' : 'zh-CN')
}

async function logout() {
  await auth.logout()
  MessagePlugin.success(t('nav.loggedOut'))
  void router.push('/login')
}
</script>

<template>
  <div v-if="bare" class="app-bare">
    <div class="app-bare__glow" aria-hidden="true" />
    <router-view />
  </div>
  <div v-else class="app-shell">
    <aside class="app-side">
      <div class="app-side__brand">
        <div class="app-side__logo" aria-hidden="true">H</div>
        <div>
          <div class="app-side__name">{{ t('app.name') }}</div>
          <div class="app-side__sub">{{ t('app.subtitle') }}</div>
        </div>
      </div>

      <div class="app-side__section">{{ t('nav.workspace') }}</div>
      <nav class="app-side__nav">
        <router-link to="/" class="app-nav__item" :class="{ 'app-nav__item--active': route.name === 'dashboard' }">
          <span class="app-nav__dot" aria-hidden="true" />
          <span>{{ t('nav.overview') }}</span>
        </router-link>
        <router-link
          to="/services"
          class="app-nav__item"
          :class="{ 'app-nav__item--active': route.name === 'services' }"
        >
          <span class="app-nav__dot" aria-hidden="true" />
          <span class="app-nav__label">{{ t('nav.services') }}</span>
          <span class="app-nav__count">{{ instances.list.length }}</span>
        </router-link>
      </nav>

      <div class="app-side__status">
        <span class="app-side__status-dot" aria-hidden="true" />
        <span>{{ t('nav.controlPlane') }}</span>
      </div>

      <div class="app-side__foot">
        <div v-if="auth.user" class="app-side__user">
          <span class="app-side__avatar" :style="{ background: auth.user.avatarColor }" aria-hidden="true">
            {{ auth.user.nickname.slice(0, 1) }}
          </span>
          <div class="app-side__username">
            {{ auth.user.nickname }}
            <UiTag v-if="auth.user.role === 'admin'" theme="primary" size="small">admin</UiTag>
          </div>
        </div>
        <UiButton variant="text" size="small" block @click="logout">{{ t('nav.logout') }}</UiButton>
      </div>
    </aside>
    <div class="app-main">
      <header class="app-top">
        <div class="app-top__crumb">
          <span>{{ t('app.name') }}</span>
          <span class="app-top__sep">/</span>
          <strong>{{ pageTitle }}</strong>
        </div>
        <button type="button" class="app-top__lang" @click="toggleLang">{{ t('lang.toggle') }}</button>
      </header>
      <main class="app-content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.app-bare {
  position: relative;
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  overflow: hidden;
  background:
    radial-gradient(50% 40% at 50% 0%, var(--mc-brand-soft), transparent 70%),
    linear-gradient(180deg, var(--td-bg-color-page), var(--td-bg-color-container));
}

.app-bare__glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image:
    linear-gradient(var(--mc-app-pattern) 1px, transparent 1px),
    linear-gradient(90deg, var(--mc-app-pattern) 1px, transparent 1px);
  background-size: 32px 32px;
  mask-image: radial-gradient(60% 60% at 50% 30%, #000, transparent 80%);
}

.app-shell {
  height: 100%;
  display: flex;
}

/* 左侧窄导航 */
.app-side {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--td-bg-color-container);
  border-right: 1px solid var(--mc-hairline);
  padding: 16px 12px;
  gap: 20px;
}

.app-side__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 4px 8px;
}

.app-side__logo {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--mc-radius-md);
  background: var(--td-brand-color);
  color: #fff;
  font-weight: 600;
  font-size: var(--mc-fs-title);
}

.app-side__name {
  font-size: var(--mc-fs-emphasis);
  font-weight: 600;
  line-height: 1.3;
}

.app-side__sub {
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-placeholder);
}

.app-side__section {
  margin: 0 8px -10px;
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-placeholder);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.app-side__nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.app-nav__item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--mc-radius-md);
  font-size: var(--mc-fs-body);
  color: var(--td-text-color-secondary);
  transition:
    background-color var(--mc-dur-instant) var(--mc-ease-out),
    color var(--mc-dur-instant) var(--mc-ease-out);
}

.app-nav__dot {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--td-text-color-placeholder);
  transition: background-color var(--mc-dur-instant) var(--mc-ease-out);
}

.app-nav__label {
  flex: 1;
  min-width: 0;
}

.app-nav__count {
  min-width: 20px;
  padding: 0 6px;
  border-radius: var(--mc-radius-pill);
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-secondary);
  font-size: var(--mc-fs-caption);
  text-align: center;
}

.app-side__status {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 6px;
  padding: 8px 10px;
  border-radius: var(--mc-radius-md);
  background: var(--mc-brand-soft);
  color: var(--td-brand-color);
  font-size: var(--mc-fs-sm);
}

.app-side__status-dot {
  width: 6px;
  height: 6px;
  flex-shrink: 0;
  border-radius: 50%;
  background: var(--td-brand-color);
  box-shadow: 0 0 0 3px var(--mc-brand-soft);
}

.app-nav__item:hover {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-primary);
}

.app-nav__item--active {
  background: var(--td-brand-color-light);
  color: var(--td-brand-color);
  font-weight: 500;
}

.app-nav__item--active .app-nav__dot {
  background: var(--td-brand-color);
}

.app-nav__item--active .app-nav__count {
  background: var(--td-bg-color-container);
  color: var(--td-brand-color);
}

.app-side__foot {
  display: flex;
  flex-direction: column;
  gap: 8px;
  border-top: 1px solid var(--mc-hairline);
  padding-top: 12px;
}

.app-side__user {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 6px;
}

.app-side__avatar {
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--mc-radius-pill);
  color: #fff;
  font-size: var(--mc-fs-sm);
}

.app-side__username {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: var(--mc-fs-body);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 主区 */
.app-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.app-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 24px;
  border-bottom: 1px solid var(--mc-hairline);
  background: var(--td-bg-color-container);
}

.app-top__crumb {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-placeholder);
}

.app-top__crumb strong {
  color: var(--td-text-color-primary);
  font-weight: 600;
}

.app-top__sep {
  color: var(--td-text-color-disabled);
}

.app-top__lang {
  border: 1px solid var(--mc-hairline);
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: var(--mc-fs-sm);
  padding: 4px 10px;
  border-radius: var(--mc-radius-sm);
  cursor: pointer;
  transition:
    background-color var(--mc-dur-instant) var(--mc-ease-out),
    color var(--mc-dur-instant) var(--mc-ease-out);
}

.app-top__lang:hover {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-primary);
}

.app-content {
  flex: 1;
  padding: 24px;
  overflow: auto;
}
</style>
