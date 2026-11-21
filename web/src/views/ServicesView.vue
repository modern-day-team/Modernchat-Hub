<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import { ApiError } from '@/api/client'
import { servicesApi, type ServiceUpsert } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { useInstancesStore } from '@/stores/instances'
import type { ServiceInstance, UpdateCheck, UpdateStatus } from '@/types'

const { t } = useI18n()
const instances = useInstancesStore()

onMounted(() => {
  void instances.fetchAll()
})

// ── 新增/编辑共用表单弹窗 ──
const dialogVisible = ref(false)
const editing = ref<ServiceInstance | null>(null) // null = 新增
const saving = ref(false)
const form = reactive({
  kind: 'generic',
  name: '',
  baseUrl: '',
  authType: 'none' as 'none' | 'bearer',
  token: '',
  healthPath: '',
  enabled: true,
  note: '',
})

/** 编辑态且已配置过凭据时，token 留空表示保持不变 */
const tokenKeepHint = computed(() => editing.value !== null && editing.value.hasToken)

function openCreate() {
  editing.value = null
  Object.assign(form, {
    kind: 'generic',
    name: '',
    baseUrl: '',
    authType: 'none' as const,
    token: '',
    healthPath: '',
    enabled: true,
    note: '',
  })
  dialogVisible.value = true
}

function openEdit(inst: ServiceInstance) {
  editing.value = inst
  Object.assign(form, {
    kind: inst.kind,
    name: inst.name,
    baseUrl: inst.baseUrl,
    authType: inst.authType,
    token: '', // 永不回填凭据明文
    healthPath: inst.healthPath,
    enabled: inst.enabled,
    note: inst.note ?? '',
  })
  dialogVisible.value = true
}

async function save() {
  if (saving.value) return
  if (!form.name.trim() || !form.baseUrl.trim()) {
    MessagePlugin.warning(t('svc.required'))
    return
  }
  saving.value = true
  const body: ServiceUpsert = {
    kind: form.kind,
    name: form.name,
    baseUrl: form.baseUrl,
    authType: form.authType,
    token: form.token || undefined,
    healthPath: form.healthPath,
    enabled: form.enabled,
    note: form.note || undefined,
  }
  try {
    if (editing.value) {
      await instances.update(editing.value.id, body)
      MessagePlugin.success(t('svc.updated'))
    } else {
      await instances.create(body)
      MessagePlugin.success(t('svc.created'))
    }
    dialogVisible.value = false
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    saving.value = false
  }
}

// ── 行内探测 ──
const checkingId = ref<number | null>(null)

async function checkRow(inst: ServiceInstance) {
  checkingId.value = inst.id
  try {
    const r = await instances.check(inst.id)
    MessagePlugin[r.lastStatus === 'ok' ? 'success' : 'warning'](
      r.lastStatus === 'ok' ? t('svc.checkedOk') : t('svc.checkedFail'),
    )
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    checkingId.value = null
  }
}

// ── 删除确认 ──
const deleteTarget = ref<ServiceInstance | null>(null)
const deleting = ref(false)

async function confirmDelete() {
  if (!deleteTarget.value || deleting.value) return
  deleting.value = true
  try {
    await instances.remove(deleteTarget.value.id)
    MessagePlugin.success(t('svc.deleted'))
    deleteTarget.value = null
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    deleting.value = false
  }
}

function statusTheme(s: ServiceInstance['lastStatus']): 'success' | 'danger' | 'default' {
  return s === 'ok' ? 'success' : s === 'fail' ? 'danger' : 'default'
}

function statusText(s: ServiceInstance['lastStatus']) {
  return s === 'ok' ? t('svc.statusOk') : s === 'fail' ? t('svc.statusFail') : t('svc.statusUnknown')
}

function fmtTime(iso?: string) {
  return iso ? new Date(iso).toLocaleString() : t('svc.notProbed')
}

// ── 更新管理（kind 有对应 adapter 时可用，经后端代理目标服务）──
const updTarget = ref<ServiceInstance | null>(null)
const updStatus = ref<UpdateStatus | null>(null)
const updCheck = ref<UpdateCheck | null>(null)
const updChecking = ref(false)
const updStarting = ref(false)
let updTimer: ReturnType<typeof setInterval> | null = null

async function refreshUpdateStatus(id: number) {
  try {
    updStatus.value = await servicesApi.updateStatus(id)
  } catch (e) {
    updStatus.value = null
    if (updTarget.value) MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  }
}

function openUpdate(inst: ServiceInstance) {
  updTarget.value = inst
  updCheck.value = null
  void refreshUpdateStatus(inst.id)
  // 更新任务是异步的：打开期间每 3s 轮询一次状态
  updTimer = setInterval(() => {
    void refreshUpdateStatus(inst.id)
  }, 3000)
}

function closeUpdate() {
  updTarget.value = null
  updStatus.value = null
  updCheck.value = null
  if (updTimer) {
    clearInterval(updTimer)
    updTimer = null
  }
}

onBeforeUnmount(closeUpdate)

async function doCheck() {
  if (!updTarget.value || updChecking.value) return
  updChecking.value = true
  try {
    updCheck.value = await servicesApi.updateCheck(updTarget.value.id)
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    updChecking.value = false
  }
}

async function doStart() {
  if (!updTarget.value || updStarting.value) return
  updStarting.value = true
  try {
    await servicesApi.updateStart(updTarget.value.id)
    MessagePlugin.success(t('upd.started'))
    void refreshUpdateStatus(updTarget.value.id)
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    updStarting.value = false
  }
}

function modeText(mode: string) {
  return mode === 'release' ? t('upd.modeRelease') : mode === 'source' ? t('upd.modeSource') : mode
}
</script>

<template>
  <div>
    <div class="svc__head">
      <h1 class="svc__title">{{ t('svc.title') }}</h1>
      <UiButton theme="primary" @click="openCreate">{{ t('svc.add') }}</UiButton>
    </div>

    <div class="svc__panel ui-panel">
      <div v-if="instances.loading && instances.list.length === 0" class="svc__empty">{{ t('common.save') }}…</div>
      <div v-else-if="instances.list.length === 0" class="svc__empty">{{ t('svc.empty') }}</div>
      <table v-else class="svc__table">
        <thead>
          <tr>
            <th>{{ t('svc.name') }}</th>
            <th>{{ t('svc.baseUrl') }}</th>
            <th>{{ t('svc.lastCheck') }}</th>
            <th class="svc__ops-col"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="inst in instances.list" :key="inst.id">
            <td>
              <div class="svc__name">
                {{ inst.name }}
                <UiTag v-if="inst.kind !== 'generic'" theme="primary" variant="outline" size="small">{{ inst.kind }}</UiTag>
                <UiTag v-if="!inst.enabled" theme="default" size="small">{{ t('svc.statusUnknown') }}</UiTag>
              </div>
              <div v-if="inst.note" class="svc__note">{{ inst.note }}</div>
            </td>
            <td>
              <div class="svc__url">{{ inst.baseUrl }}</div>
              <div class="svc__meta">
                <UiTag theme="default" variant="outline" size="small">{{ inst.authType === 'bearer' ? t('svc.authBearer') : t('svc.authNone') }}</UiTag>
                <span class="svc__path">{{ inst.healthPath }}</span>
              </div>
            </td>
            <td>
              <UiTag :theme="statusTheme(inst.lastStatus)">{{ statusText(inst.lastStatus) }}</UiTag>
              <div class="svc__meta">
                <span>{{ t('svc.version') }}: {{ inst.lastVersion ?? '—' }}</span>
                <span class="svc__time">{{ fmtTime(inst.lastCheckAt) }}</span>
              </div>
            </td>
            <td class="svc__ops">
              <UiButton variant="outline" size="small" :loading="checkingId === inst.id" @click="checkRow(inst)">
                {{ checkingId === inst.id ? t('svc.checking') : t('svc.check') }}
              </UiButton>
              <UiButton
                v-if="inst.kind === 'modernchat-new'"
                variant="text"
                theme="primary"
                size="small"
                @click="openUpdate(inst)"
              >
                {{ t('svc.rowUpdate') }}
              </UiButton>
              <UiButton variant="text" size="small" @click="openEdit(inst)">{{ t('svc.rowEdit') }}</UiButton>
              <UiButton variant="text" theme="danger" size="small" @click="deleteTarget = inst">
                {{ t('svc.rowDelete') }}
              </UiButton>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 新增/编辑弹窗 -->
    <UiDialog
      v-model:visible="dialogVisible"
      :header="editing ? t('svc.edit') : t('svc.add')"
      :width="460"
      :confirm-text="t('common.save')"
      :confirm-loading="saving"
      :cancel-text="t('common.cancel')"
      @confirm="save"
    >
      <form class="svc-form" @submit.prevent>
        <div class="svc-form__field">
          <span class="ui-label">{{ t('svc.kind') }}</span>
          <select v-model="form.kind" class="svc-form__select">
            <option value="generic">{{ t('svc.kindGeneric') }}</option>
            <option value="modernchat-new">{{ t('svc.kindModernchatNew') }}</option>
          </select>
          <span class="ui-hint">{{ t('svc.kindHint') }}</span>
        </div>
        <label class="svc-form__field">
          <span class="ui-label">{{ t('svc.name') }}</span>
          <UiInput v-model="form.name" :placeholder="t('svc.namePlaceholder')" :maxlength="30" clearable />
        </label>
        <label class="svc-form__field">
          <span class="ui-label">{{ t('svc.baseUrl') }}</span>
          <UiInput v-model="form.baseUrl" :placeholder="t('svc.baseUrlPlaceholder')" clearable />
        </label>
        <div class="svc-form__field">
          <span class="ui-label">{{ t('svc.authType') }}</span>
          <select v-model="form.authType" class="svc-form__select">
            <option value="none">{{ t('svc.authNone') }}</option>
            <option value="bearer">{{ t('svc.authBearer') }}</option>
          </select>
        </div>
        <label v-if="form.authType === 'bearer'" class="svc-form__field">
          <span class="ui-label">{{ t('svc.token') }}</span>
          <UiInput v-model="form.token" type="password" :placeholder="t('svc.token')" clearable />
          <span class="ui-hint">{{ tokenKeepHint ? t('svc.tokenKeepHint') : t('svc.tokenNewHint') }}</span>
        </label>
        <label class="svc-form__field">
          <span class="ui-label">{{ t('svc.healthPath') }}</span>
          <UiInput v-model="form.healthPath" :placeholder="t('svc.healthPathPlaceholder')" clearable />
        </label>
        <div class="svc-form__field">
          <UiCheckbox v-model="form.enabled">{{ t('svc.enabled') }}</UiCheckbox>
        </div>
        <label class="svc-form__field">
          <span class="ui-label">{{ t('svc.note') }}</span>
          <UiInput v-model="form.note" :placeholder="t('svc.notePlaceholder')" clearable />
        </label>
      </form>
    </UiDialog>

    <!-- 删除确认 -->
    <UiDialog
      :visible="deleteTarget !== null"
      :header="t('svc.deleteTitle')"
      :width="380"
      confirm-theme="danger"
      :confirm-text="t('common.confirm')"
      :confirm-loading="deleting"
      :cancel-text="t('common.cancel')"
      @update:visible="(v: boolean) => { if (!v) deleteTarget = null }"
      @confirm="confirmDelete"
      @close="deleteTarget = null"
    >
      {{ deleteTarget ? t('svc.deleteMsg', { name: deleteTarget.name }) : '' }}
    </UiDialog>

    <!-- 更新管理（经后端 adapter 代理目标服务的在线更新） -->
    <UiDialog
      :visible="updTarget !== null"
      :header="t('upd.title')"
      :width="520"
      :footer="false"
      @update:visible="(v: boolean) => { if (!v) closeUpdate() }"
      @close="closeUpdate"
    >
      <div v-if="updTarget" class="upd">
        <div class="upd__row">
          <span class="svc__name">{{ updTarget.name }}</span>
          <UiTag v-if="updTarget.lastVersion" theme="default" variant="outline" size="small">
            {{ t('svc.version') }} {{ updTarget.lastVersion }}
          </UiTag>
        </div>
        <div class="upd__row">
          <span class="ui-label">{{ t('upd.status') }}</span>
          <UiTag
            v-if="updStatus"
            :theme="!updStatus.available ? 'default' : updStatus.busy ? 'warning' : 'success'"
            size="small"
          >
            {{ !updStatus.available ? t('upd.unsupportedShort') : updStatus.busy ? t('upd.busy') : t('upd.idle') }}
          </UiTag>
          <span v-if="updStatus?.mode" class="upd__mode">{{ modeText(updStatus.mode) }}</span>
          <span class="upd__phase">{{ updStatus?.phase }}</span>
        </div>
        <div v-if="updStatus?.error" class="upd__error">{{ updStatus.error }}</div>
        <div v-if="updCheck" class="upd__check">
          <span>{{ t('upd.current') }}: {{ updCheck.currentVersion || '—' }}</span>
          <span>{{ t('upd.latest') }}: {{ updCheck.latestVersion || '—' }}</span>
          <UiTag :theme="updCheck.updateAvailable ? 'warning' : 'success'" size="small">
            {{ updCheck.updateAvailable ? t('upd.available') : t('upd.upToDate') }}
          </UiTag>
        </div>
        <div v-if="updStatus?.lines?.length" class="upd__lines">
          <div v-for="(line, i) in updStatus.lines" :key="i" class="upd__line">{{ line }}</div>
        </div>
        <div class="upd__ops">
          <UiButton variant="outline" size="small" :loading="updChecking" @click="doCheck">
            {{ t('upd.check') }}
          </UiButton>
          <UiButton theme="primary" size="small" :loading="updStarting" @click="doStart">
            {{ t('upd.start') }}
          </UiButton>
        </div>
        <p class="ui-hint">{{ t('upd.hint') }}</p>
      </div>
    </UiDialog>
  </div>
</template>

<style scoped>
.svc__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}

.svc__title {
  margin: 0;
  font-size: var(--mc-fs-title);
  font-weight: 600;
}

.svc__panel {
  padding: 8px 16px;
}

.svc__empty {
  padding: 32px 8px;
  text-align: center;
  color: var(--td-text-color-secondary);
}

.svc__table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--mc-fs-body);
}

.svc__table th {
  text-align: left;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  padding: 10px 8px;
  border-bottom: 1px solid var(--mc-hairline);
}

.svc__table td {
  padding: 10px 8px;
  border-bottom: 1px solid var(--mc-hairline);
  vertical-align: top;
}

.svc__table tr:last-child td {
  border-bottom: none;
}

.svc__name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

.svc__note {
  margin-top: 2px;
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-placeholder);
}

.svc__url {
  word-break: break-all;
}

.svc__meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 4px;
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-secondary);
}

.svc__path,
.svc__time {
  color: var(--td-text-color-placeholder);
}

.svc__ops {
  text-align: right;
  white-space: nowrap;
}

.svc__ops .ui-btn + .ui-btn {
  margin-left: 4px;
}

/* 表单 */
.svc-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.svc-form__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.svc-form__select {
  height: 32px;
  padding: 0 8px;
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-md);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: var(--mc-fs-body);
  outline: none;
  cursor: pointer;
}

.svc-form__select:focus {
  border-color: var(--td-brand-color);
}

/* 更新管理对话框 */
.upd {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.upd__row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--mc-fs-body);
}

.upd__mode {
  color: var(--td-text-color-secondary);
  font-size: var(--mc-fs-sm);
}

.upd__phase {
  color: var(--td-text-color-placeholder);
  font-size: var(--mc-fs-sm);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.upd__error {
  padding: 8px 10px;
  border-radius: var(--mc-radius-sm);
  background: var(--td-error-color);
  color: #fff;
  font-size: var(--mc-fs-sm);
  word-break: break-all;
}

.upd__check {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-md);
  font-size: var(--mc-fs-body);
}

.upd__lines {
  max-height: 180px;
  overflow: auto;
  padding: 8px 10px;
  border-radius: var(--mc-radius-md);
  background: var(--td-bg-color-page);
  font-family: ui-monospace, Consolas, monospace;
  font-size: var(--mc-fs-sm);
  line-height: 1.6;
  color: var(--td-text-color-secondary);
}

.upd__line {
  white-space: pre-wrap;
  word-break: break-all;
}

.upd__ops {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
</style>
