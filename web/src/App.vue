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
    <!-- 认证卡片限宽走五档容器里的 narrow（--mc-container-narrow） -->
    <div class="mc-container mc-container--narrow">
      <router-view />
    </div>
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
      <!-- 页面骨架（限宽、留白、区块节奏）由各视图根节点的 .mc-page--wide 负责 -->
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
  /* 左右留白交给 .mc-container（= 槽宽），这里只给上下留白，免得窄屏两侧叠成双份 */
  padding: var(--mc-page-y) 0;
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

/* 面板壳。移动优先：<768 时侧栏横排在顶部（品牌 + 退出登录一行、导航横排一行），
   不引入抽屉状态与新依赖；≥768 才回到左侧竖栏。 */
.app-shell {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.app-side {
  flex-shrink: 0;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--mc-sp-3);
  padding: var(--mc-sp-3) var(--mc-gutter);
  background: var(--td-bg-color-container);
  border-bottom: 1px solid var(--mc-hairline);
}

.app-side__brand {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: var(--mc-sp-2);
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
  color: var(--td-text-color-anti);
  font-weight: 600;
  font-size: var(--mc-fs-title);
}

.app-side__name {
  font-size: var(--mc-fs-emphasis);
  font-weight: 600;
  line-height: var(--mc-lh-snug);
}

.app-side__sub {
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-placeholder);
}

/* 「工作区」小标题只服务竖栏，顶部栏里没有它的位置 */
.app-side__section {
  display: none;
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-placeholder);
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.app-side__nav {
  /* 顶部栏里导航独占第二行 */
  order: 3;
  flex-basis: 100%;
  display: flex;
  flex-wrap: wrap;
  gap: var(--mc-sp-1);
}

.app-nav__item {
  display: flex;
  align-items: center;
  gap: var(--mc-sp-2);
  padding: var(--mc-sp-2) var(--mc-sp-3);
  border-radius: var(--mc-radius-md);
  font-size: var(--mc-fs-body);
  line-height: var(--mc-lh-snug);
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
  line-height: var(--mc-lh-snug);
  text-align: center;
}

.app-side__status {
  display: none;
  align-items: center;
  gap: var(--mc-sp-2);
  margin: 0 var(--mc-sp-1);
  padding: var(--mc-sp-2) var(--mc-sp-3);
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
  align-items: center;
  gap: var(--mc-sp-2);
}

/* 用户信息只在竖栏里展示；顶部栏空间有限，保留退出登录即可 */
.app-side__user {
  display: none;
  align-items: center;
  gap: var(--mc-sp-2);
  min-width: 0;
  padding: 0 var(--mc-sp-2);
}

.app-side__avatar {
  width: 26px;
  height: 26px;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--mc-radius-pill);
  color: var(--td-text-color-anti);
  font-size: var(--mc-fs-sm);
}

.app-side__username {
  display: flex;
  align-items: center;
  gap: var(--mc-sp-2);
  font-size: var(--mc-fs-body);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ≥768：侧栏回到左侧竖栏。
   宽度取 224：内边距 12×2 后内容宽 200，放得下「用户名 + admin 标签」；
   同时是 8 与 16 的整倍数（200 是原值，英文导航 + 计数徽标偏挤）。 */
@media (min-width: 768px) {
  .app-shell {
    flex-direction: row;
  }

  .app-side {
    width: 224px;
    height: 100%;
    flex-wrap: nowrap;
    flex-direction: column;
    align-items: stretch;
    gap: var(--mc-sp-5);
    padding: var(--mc-sp-4) var(--mc-sp-3);
    border-bottom: none;
    border-right: 1px solid var(--mc-hairline);
  }

  .app-side__brand {
    padding: var(--mc-sp-1) var(--mc-sp-2);
  }

  .app-side__section {
    display: block;
    margin: 0 var(--mc-sp-2) calc(var(--mc-sp-2) * -1);
  }

  .app-side__nav {
    order: 0;
    flex: 1;
    flex-basis: auto;
    flex-direction: column;
    flex-wrap: nowrap;
    gap: var(--mc-sp-1);
  }

  .app-side__status {
    display: flex;
  }

  .app-side__foot {
    flex-direction: column;
    align-items: stretch;
    gap: var(--mc-sp-2);
    border-top: 1px solid var(--mc-hairline);
    padding-top: var(--mc-sp-3);
  }

  .app-side__user {
    display: flex;
  }
}

/* 主区 */
.app-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

/* 顶栏：高度 56（= 48 + 8，4pt 对齐），左右留白 = 槽宽 */
.app-top {
  flex-shrink: 0;
  height: calc(var(--mc-sp-12) + var(--mc-sp-2));
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--mc-sp-3);
  padding: 0 var(--mc-gutter);
  border-bottom: 1px solid var(--mc-hairline);
  background: var(--td-bg-color-container);
}

.app-top__crumb {
  display: flex;
  align-items: center;
  gap: var(--mc-sp-2);
  min-width: 0;
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-placeholder);
}

/* <768 顶栏上方就是带品牌的顶部栏，面包屑只留当前页名 */
.app-top__crumb > span {
  display: none;
}

@media (min-width: 768px) {
  .app-top__crumb > span {
    display: inline;
  }
}

.app-top__crumb strong {
  color: var(--td-text-color-primary);
  font-weight: 600;
}

.app-top__sep {
  color: var(--td-text-color-disabled);
}

.app-top__lang {
  height: var(--mc-ctl-sm);
  border: 1px solid var(--mc-hairline);
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: var(--mc-fs-sm);
  padding: 0 var(--mc-sp-3);
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

/* 留白与限宽交给视图根节点的 .mc-page--wide */
.app-content {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
</style>
