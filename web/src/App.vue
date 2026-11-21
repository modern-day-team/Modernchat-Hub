<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import UiButton from '@/components/ui/UiButton.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { setLang } from '@/i18n'
import { useAuthStore } from '@/stores/auth'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

// 登录/注册页不套面板壳，居中卡片呈现
const bare = computed(() => route.name === 'login' || route.name === 'register')

onMounted(() => {
  // 刷新后补拉当前用户（401 已由 client 统一处理）
  void auth.fetchMe()
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
      <nav class="app-side__nav">
        <router-link to="/" class="app-nav__item" :class="{ 'app-nav__item--active': route.name === 'dashboard' }">
          {{ t('nav.overview') }}
        </router-link>
        <router-link
          to="/services"
          class="app-nav__item"
          :class="{ 'app-nav__item--active': route.name === 'services' }"
        >
          {{ t('nav.services') }}
        </router-link>
      </nav>
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
        <span />
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
  min-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
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

.app-side__nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.app-nav__item {
  display: block;
  padding: 8px 10px;
  border-radius: var(--mc-radius-md);
  font-size: var(--mc-fs-body);
  color: var(--td-text-color-secondary);
  transition:
    background-color var(--mc-dur-instant) var(--mc-ease-out),
    color var(--mc-dur-instant) var(--mc-ease-out);
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
