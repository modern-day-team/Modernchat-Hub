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
  auth
    .login(form.username, form.password)
    .then(afterLogin)
    .catch((e: unknown) => {
      MessagePlugin.error(e instanceof ApiError ? e.message : t('auth.failed'))
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
        <UiInput v-model="form.username" :placeholder="t('auth.username')" clearable />
      </TFormItem>
      <TFormItem name="password">
        <UiInput v-model="form.password" type="password" :placeholder="t('auth.password')" clearable />
      </TFormItem>
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
.auth-card {
  width: 100%;
  max-width: 400px;
  background: var(--td-bg-color-container);
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-lg);
  box-shadow: var(--mc-elev-2);
  padding: 28px 28px 24px;
}

.auth-card__brand {
  display: flex;
  align-items: center;
  gap: 10px;
  padding-bottom: 18px;
  margin-bottom: 20px;
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
  color: #fff;
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
  margin-bottom: 20px;
}

.auth-card__title {
  font-size: var(--mc-fs-heading);
  font-weight: 600;
  margin: 0 0 6px;
}

.auth-card__subtitle {
  margin: 0;
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-secondary);
}

.auth-card__divider {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 20px 0 12px;
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
