<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

import UiButton from './UiButton.vue'

/**
 * 模态对话框（UnoCSS 实现的家族风格精简版，替代 TDesign Dialog）。
 *
 * - `v-model:visible`、`header`、`width`；`footer=false` 为纯展示弹窗
 * - `confirmTheme=danger` 用于删除确认；`confirmLoading` 期间禁止重复提交
 * - 关闭途径：右上角 ×、点遮罩、Esc；打开时锁 body 滚动，关闭后把焦点还给打开前的元素
 */
const props = withDefaults(
  defineProps<{
    visible?: boolean
    header?: string
    width?: string | number
    /** false 时不渲染底部按钮区（纯展示型弹窗） */
    footer?: boolean
    confirmTheme?: 'primary' | 'danger'
    confirmText?: string
    confirmLoading?: boolean
    cancelText?: string
  }>(),
  {
    visible: false,
    header: '',
    width: 480,
    footer: true,
    confirmTheme: 'primary',
    confirmText: '',
    confirmLoading: false,
    cancelText: '',
  },
)

const emit = defineEmits<{
  'update:visible': [value: boolean]
  confirm: []
  close: []
}>()

const panelEl = ref<HTMLElement | null>(null)
let restoreFocus: HTMLElement | null = null
let scrollLocked = false

const widthStyle = computed(() => (typeof props.width === 'number' ? `${props.width}px` : props.width))

function close() {
  emit('update:visible', false)
  emit('close')
}

function onConfirm() {
  if (props.confirmLoading) return
  emit('confirm')
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

function lockScroll() {
  if (scrollLocked) return
  scrollLocked = true
  document.body.style.overflow = 'hidden'
}

function unlockScroll() {
  if (!scrollLocked) return
  scrollLocked = false
  document.body.style.overflow = ''
}

watch(
  () => props.visible,
  async (v) => {
    if (v) {
      restoreFocus = document.activeElement as HTMLElement | null
      lockScroll()
      await nextTick()
      panelEl.value?.focus()
    } else {
      unlockScroll()
      restoreFocus?.focus()
      restoreFocus = null
    }
  },
)

onBeforeUnmount(unlockScroll)
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="ui-dialog__overlay"
      @keydown="onKeydown"
      @click.self="close"
    >
      <div
        ref="panelEl"
        class="ui-dialog__panel"
        :style="{ width: widthStyle }"
        role="dialog"
        aria-modal="true"
        :aria-label="header"
        tabindex="-1"
      >
        <div class="ui-dialog__head">
          <h2 class="ui-dialog__title">{{ header }}</h2>
          <button type="button" class="ui-dialog__close" aria-label="close" @click="close">×</button>
        </div>
        <div class="ui-dialog__body">
          <slot />
        </div>
        <div v-if="footer" class="ui-dialog__foot">
          <UiButton variant="outline" @click="close">{{ cancelText }}</UiButton>
          <UiButton :theme="confirmTheme" :loading="confirmLoading" @click="onConfirm">
            {{ confirmText }}
          </UiButton>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.ui-dialog__overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(0, 0, 0, 0.4);
}

.ui-dialog__panel {
  max-width: calc(100vw - 48px);
  max-height: calc(100vh - 48px);
  overflow: auto;
  background: var(--td-bg-color-container);
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-xl);
  box-shadow: var(--mc-elev-3);
  outline: none;
  animation: ui-dialog-in var(--mc-dur-base) var(--mc-ease-out);
}

@keyframes ui-dialog-in {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.98);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .ui-dialog__panel {
    animation: none;
  }
}

.ui-dialog__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 24px 0;
}

.ui-dialog__title {
  margin: 0;
  font-size: var(--mc-fs-title);
  font-weight: 600;
}

.ui-dialog__close {
  border: none;
  background: transparent;
  color: var(--td-text-color-secondary);
  font-size: 20px;
  line-height: 1;
  width: 28px;
  height: 28px;
  border-radius: var(--mc-radius-sm);
  cursor: pointer;
}

.ui-dialog__close:hover {
  background: var(--td-bg-color-container-hover);
  color: var(--td-text-color-primary);
}

.ui-dialog__body {
  padding: 16px 24px;
  font-size: var(--mc-fs-body);
  color: var(--td-text-color-primary);
}

.ui-dialog__foot {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 0 24px 20px;
}
</style>
