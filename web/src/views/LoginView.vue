<script setup lang="ts">
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Form as TForm, FormItem as TFormItem, MessagePlugin } from 'tdesign-vue-next'
import type { FormProps } from 'tdesign-vue-next'

import { ApiError } from '@/api/client'
import UiButton from '@/components/ui/UiButton.vue'
import UiInput from '@/components/ui/UiInput.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const auth = useAuthStore()

const form = reactive({ username: '', password: '' })
const loading = ref(false)
/* 提交失败的内联错误：比 toast 更靠近表单且不自动消失（toast 仍保留给全局观察） */
const error = ref('')

const rules: FormProps['rules'] = {
  username: [{ required: true, message: t('auth.required') }],
  password: [{ required: true, message: t('auth.required') }],
}

/** 登录成功后回来源页（client 在 401 时会带 ?redirect=... 回来） */
function afterLogin() {
  const q = route.query.redirect
  const redirect = typeof q === 'string' && q.startsWith('/') ? q : '/'
  void router.push(redirect)
}

/** TForm 的 submit：Enter 与按钮统一走校验→提交（沿用项目组 AuthView 模式） */
function onSubmit({ validateResult }: { validateResult: boolean | Record<string, unknown> }) {
  if (validateResult !== true || loading.value) return
  loading.value = true
  error.value = ''
  auth
    .login(form.username, form.password)
    .then(afterLogin)
    .catch((e: unknown) => {
      error.value = e instanceof ApiError ? e.message : t('auth.failed')
      MessagePlugin.error(error.value)
    })
    .finally(() => {
      loading.value = false
    })
}
</script>

<template>
  <main class="auth-card">
    <div class="auth-card__brand">
      <span class="auth-card__logo" aria-hidden="true">H</span>
      <div>
        <div class="auth-card__app">{{ t('app.name') }}</div>
        <div class="auth-card__subtitle-app">{{ t('app.subtitle') }}</div>
      </div>
    </div>

    <header class="auth-card__header">
      <h1 class="auth-card__title">{{ t('auth.welcomeBack') }}</h1>
      <p class="auth-card__subtitle">{{ t('auth.loginSubtitle') }}</p>
    </header>

    <TForm :data="form" :rules="rules" label-width="0" @submit="onSubmit">
      <TFormItem name="username">
        <UiInput v-model="form.username" :placeholder="t('auth.username')" :aria-label="t('auth.username')" clearable />
      </TFormItem>
      <TFormItem name="password">
        <UiInput v-model="form.password" type="password" :placeholder="t('auth.password')" :aria-label="t('auth.password')" clearable />
      </TFormItem>
      <!-- 提交失败的持久内联提示（错误说明靠近表单，不随 toast 消失） -->
      <p v-if="error" class="auth-card__error" role="alert">{{ error }}</p>
      <UiButton theme="primary" type="submit" block :loading="loading">
        {{ t('auth.login') }}
      </UiButton>
    </TForm>

    <div class="auth-card__divider">
      <span>{{ t('auth.noAccount') }}</span>
    </div>
    <router-link to="/register" class="auth-card__link">{{ t('auth.toRegister') }}</router-link>
  </main>
</template>

<style scoped>
/* 卡片宽度由 App.vue 外层的 .mc-container--narrow（400 档容器）统一限宽 */
.auth-card {
  width: 100%;
  background: var(--td-bg-color-container);
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-lg);
  box-shadow: var(--mc-elev-2);
  padding: var(--mc-sp-6) var(--mc-sp-6) var(--mc-sp-5);
}

.auth-card__brand {
  display: flex;
  align-items: center;
  gap: var(--mc-sp-2);
  padding-bottom: var(--mc-sp-4);
  margin-bottom: var(--mc-sp-5);
  border-bottom: 1px solid var(--mc-hairline);
}

.auth-card__logo {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border-radius: var(--mc-radius-md);
  background: var(--td-brand-color);
  color: var(--td-text-color-anti);
  font-size: var(--mc-fs-title);
  font-weight: 600;
}

.auth-card__app {
  font-size: var(--mc-fs-emphasis);
  font-weight: 600;
  line-height: 1.3;
}

.auth-card__subtitle-app {
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-placeholder);
}

.auth-card__header {
  margin-bottom: var(--mc-sp-5);
}

.auth-card__title {
  font-size: var(--mc-fs-heading);
  font-weight: 600;
  letter-spacing: -0.02em;
  line-height: var(--mc-lh-tight);
  margin: 0 0 var(--mc-sp-1);
}

.auth-card__subtitle {
  margin: 0;
  font-size: var(--mc-fs-sm);
  line-height: var(--mc-lh-normal);
  color: var(--td-text-color-secondary);
}

.auth-card__error {
  margin: 0 0 var(--mc-sp-3);
  padding: var(--mc-sp-2) var(--mc-sp-3);
  border-radius: var(--mc-radius-sm);
  background: var(--td-error-color-1, rgba(227, 77, 89, 0.08));
  color: var(--td-error-color);
  font-size: var(--mc-fs-sm);
  line-height: var(--mc-lh-normal);
}

.auth-card__divider {
  display: flex;
  align-items: center;
  gap: var(--mc-sp-2);
  margin: var(--mc-sp-5) 0 var(--mc-sp-3);
  color: var(--td-text-color-placeholder);
  font-size: var(--mc-fs-sm);
}

.auth-card__divider::before,
.auth-card__divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--mc-hairline);
}

.auth-card__link {
  display: block;
  text-align: center;
  font-size: var(--mc-fs-sm);
  font-weight: 500;
}
</style>
