<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { MessagePlugin } from 'tdesign-vue-next'

import { ApiError } from '@/api/client'
import { servicesApi } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import type { ServiceInstance, TargetAdminStats, TargetAdminUser } from '@/types'

/**
 * 远程管理对话框（统一管理后台）：经后端 adapter 代理目标服务的站长 API。
 * 打开即拉概览统计 + 用户分页；封禁/解封/强制下线/重置密码为显式按钮操作。
 * 重置密码的一次性明文只在本对话框出现，不落库不进日志。
 */
const props = defineProps<{
  visible?: boolean
  instance?: ServiceInstance | null
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { t } = useI18n()

const stats = ref<TargetAdminStats | null>(null)
const users = ref<TargetAdminUser[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = 20
const kw = ref('')
const role = ref('')
const status = ref('')
const loading = ref(false)

const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))

const statTiles = computed(() => [
  { label: t('adm.total'), value: stats.value?.usersTotal },
  { label: t('adm.today'), value: stats.value?.usersToday },
  { label: t('adm.bannedTile'), value: stats.value?.usersBanned },
  { label: t('adm.admins'), value: stats.value?.usersAdmins },
  { label: t('adm.convs'), value: stats.value?.conversations },
  { label: t('adm.msgs'), value: stats.value?.messagesTotal },
  { label: t('adm.online'), value: stats.value?.online },
  { label: t('adm.uptime'), value: uptimeText(stats.value?.uptimeSeconds) },
])

watch(
  () => props.visible,
  (v) => {
    if (v) {
      page.value = 1
      void reload()
    }
  },
)

async function reload() {
  if (!props.instance) return
  loading.value = true
  try {
    stats.value = await servicesApi.adminStats(props.instance.id)
    const p = await servicesApi.adminUsers(props.instance.id, {
      page: page.value,
      pageSize,
      kw: kw.value || undefined,
      role: role.value || undefined,
      status: status.value || undefined,
    })
    users.value = p.users
    total.value = p.total
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    loading.value = false
  }
}

function search() {
  page.value = 1
  void reload()
}

function goto(p: number) {
  page.value = p
  void reload()
}

function close() {
  emit('update:visible', false)
}

// ── 封禁子弹窗 ──
const banTarget = ref<TargetAdminUser | null>(null)
const banDays = ref('7')
const banReason = ref('')
const banning = ref(false)

function openBan(u: TargetAdminUser) {
  banTarget.value = u
  banDays.value = '7'
  banReason.value = ''
}

async function confirmBan() {
  if (!banTarget.value || banning.value || !props.instance) return
  const days = Number.parseInt(banDays.value, 10)
  if (Number.isNaN(days) || days < 0 || days > 3650) {
    MessagePlugin.warning(t('adm.badDays'))
    return
  }
  banning.value = true
  try {
    await servicesApi.adminBan(props.instance.id, banTarget.value.id, { days, reason: banReason.value || undefined })
    MessagePlugin.success(t('adm.banDone', { name: banTarget.value.username }))
    banTarget.value = null
    void reload()
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    banning.value = false
  }
}

// ── 解封 / 强制下线 ──
const acting = ref(false)

async function doUnban(u: TargetAdminUser) {
  if (!props.instance || acting.value) return
  acting.value = true
  try {
    await servicesApi.adminUnban(props.instance.id, u.id)
    MessagePlugin.success(t('adm.unbanDone', { name: u.username }))
    void reload()
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    acting.value = false
  }
}

async function doKick(u: TargetAdminUser) {
  if (!props.instance || acting.value) return
  acting.value = true
  try {
    const r = await servicesApi.adminKick(props.instance.id, u.id)
    MessagePlugin.success(r.wasOnline ? t('adm.kickedOnline', { name: u.username }) : t('adm.kickedOffline', { name: u.username }))
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    acting.value = false
  }
}

// ── 重置密码结果（一次性明文只在这里出现） ──
const resetUser = ref<TargetAdminUser | null>(null)
const resetPwd = ref('')

async function doReset(u: TargetAdminUser) {
  if (!props.instance || acting.value) return
  acting.value = true
  try {
    const r = await servicesApi.adminResetPassword(props.instance.id, u.id)
    resetUser.value = u
    resetPwd.value = r.newPassword
  } catch (e) {
    MessagePlugin.error(e instanceof ApiError ? e.message : t('svc.failed'))
  } finally {
    acting.value = false
  }
}

function isBanned(u: TargetAdminUser) {
  return !!u.bannedUntil
}

function fmtDate(iso?: string) {
  return iso ? new Date(iso).toLocaleString() : '—'
}

function uptimeText(sec?: number) {
  if (sec === undefined || sec === null) return '—'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  return d > 0 ? `${d}d ${h}h` : h > 0 ? `${h}h ${m}m` : `${m}m`
}
</script>

<template>
  <UiDialog
    :visible="props.visible"
    :header="`${t('adm.title')} · ${instance?.name ?? ''}`"
    :width="760"
    :footer="false"
    @update:visible="(v: boolean) => { if (!v) close() }"
    @close="close"
  >
    <div v-if="instance">
      <!-- 概览统计 -->
      <div class="adm__stats">
        <div v-for="tile in statTiles" :key="tile.label" class="adm__tile">
          <div class="adm__num">{{ tile.value ?? '—' }}</div>
          <div class="adm__label">{{ tile.label }}</div>
        </div>
      </div>

      <!-- 筛选工具条 -->
      <div class="adm__toolbar">
        <UiInput
          v-model="kw"
          :placeholder="t('adm.search')"
          size="small"
          clearable
          @enter="search"
        />
        <select v-model="role" class="adm__select" @change="search">
          <option value="">{{ t('adm.allRoles') }}</option>
          <option value="user">{{ t('adm.roleUser') }}</option>
          <option value="admin">{{ t('adm.roleAdmin') }}</option>
        </select>
        <select v-model="status" class="adm__select" @change="search">
          <option value="">{{ t('adm.allStatus') }}</option>
          <option value="active">{{ t('adm.stActive') }}</option>
          <option value="banned">{{ t('adm.stBanned') }}</option>
        </select>
        <UiButton variant="outline" size="small" @click="search">{{ t('adm.searchBtn') }}</UiButton>
      </div>

      <!-- 用户表 -->
      <table class="adm__table">
        <thead>
          <tr>
            <th>{{ t('adm.colUser') }}</th>
            <th>{{ t('adm.colRole') }}</th>
            <th>{{ t('adm.colEmail') }}</th>
            <th>{{ t('adm.colStatus') }}</th>
            <th>{{ t('adm.colCreatedAt') }}</th>
            <th class="adm__ops-col"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading"><td colspan="6" class="adm__empty">{{ t('adm.loading') }}</td></tr>
          <tr v-else-if="users.length === 0"><td colspan="6" class="adm__empty">{{ t('adm.empty') }}</td></tr>
          <template v-else>
            <tr v-for="u in users" :key="u.id">
              <td>
                <div class="adm__username">{{ u.nickname }} <span class="adm__uname">@{{ u.username }}</span></div>
                <div v-if="u.banReason" class="adm__reason">{{ u.banReason }}</div>
              </td>
              <td>
                <UiTag :theme="u.role === 'admin' ? 'primary' : 'default'" size="small">
                  {{ u.role === 'admin' ? t('adm.roleAdmin') : t('adm.roleUser') }}
                </UiTag>
              </td>
              <td class="adm__muted">{{ u.email || '—' }}</td>
              <td>
                <UiTag v-if="isBanned(u)" theme="danger" size="small">{{ t('adm.stBanned') }}</UiTag>
                <span v-if="isBanned(u)" class="adm__until">{{ t('adm.bannedUntil') }} {{ fmtDate(u.bannedUntil) }}</span>
                <span v-else class="adm__muted">{{ t('adm.stActive') }}</span>
              </td>
              <td class="adm__muted">{{ fmtDate(u.createdAt) }}</td>
              <td class="adm__ops">
                <UiButton v-if="!isBanned(u)" variant="text" theme="danger" size="small" @click="openBan(u)">
                  {{ t('adm.ban') }}
                </UiButton>
                <UiButton v-else variant="text" size="small" :disabled="acting" @click="doUnban(u)">
                  {{ t('adm.unban') }}
                </UiButton>
                <UiButton variant="text" size="small" :disabled="acting" @click="doKick(u)">
                  {{ t('adm.kick') }}
                </UiButton>
                <UiButton variant="text" size="small" :disabled="acting" @click="doReset(u)">
                  {{ t('adm.resetPwd') }}
                </UiButton>
              </td>
            </tr>
          </template>
        </tbody>
      </table>

      <!-- 分页 -->
      <div class="adm__pager">
        <span class="adm__muted">{{ t('adm.totalN', { n: total }) }}</span>
        <span>
          <UiButton variant="outline" size="small" :disabled="page <= 1 || loading" @click="goto(page - 1)">
            {{ t('adm.prev') }}
          </UiButton>
          <span class="adm__page">{{ page }} / {{ totalPages }}</span>
          <UiButton variant="outline" size="small" :disabled="page >= totalPages || loading" @click="goto(page + 1)">
            {{ t('adm.next') }}
          </UiButton>
        </span>
      </div>
    </div>
  </UiDialog>

  <!-- 封禁确认子弹窗 -->
  <UiDialog
    :visible="banTarget !== null"
    :header="`${t('adm.banTitle')}${banTarget ? ' @' + banTarget.username : ''}`"
    :width="400"
    :confirm-text="t('adm.ban')"
    confirm-theme="danger"
    :confirm-loading="banning"
    :cancel-text="t('common.cancel')"
    @update:visible="(v: boolean) => { if (!v) banTarget = null }"
    @close="banTarget = null"
    @confirm="confirmBan"
  >
    <div class="adm__banform">
      <label class="adm__field">
        <span class="ui-label">{{ t('adm.banDays') }}</span>
        <UiInput v-model="banDays" :maxlength="4" />
      </label>
      <label class="adm__field">
        <span class="ui-label">{{ t('adm.banReason') }}</span>
        <UiInput v-model="banReason" :maxlength="200" clearable />
      </label>
    </div>
  </UiDialog>

  <!-- 重置密码结果子弹窗（一次性明文只在这里出现） -->
  <UiDialog
    :visible="resetPwd !== ''"
    :header="t('adm.resetTitle')"
    :width="380"
    :confirm-text="t('common.confirm')"
    :cancel-text="t('common.cancel')"
    @update:visible="(v: boolean) => { if (!v) { resetPwd = ''; resetUser = null } }"
    @close="resetPwd = ''"
    @confirm="resetPwd = ''"
  >
    <p class="adm__resetmsg">
      {{ resetUser ? t('adm.resetFor', { name: resetUser.username }) : '' }}
    </p>
    <div class="adm__pwd">{{ resetPwd }}</div>
    <p class="ui-hint">{{ t('adm.resetHint') }}</p>
  </UiDialog>
</template>

<style scoped>
.adm__stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 14px;
}

.adm__tile {
  background: var(--td-bg-color-page);
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-md);
  padding: 10px 12px;
}

.adm__num {
  font-size: var(--mc-fs-title);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.adm__label {
  margin-top: 2px;
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-secondary);
}

.adm__toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
}

.adm__toolbar :deep(.ui-input-wrap, input) {
  flex: 1;
}

.adm__select {
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-md);
  background: var(--td-bg-color-container);
  color: var(--td-text-color-primary);
  font-size: var(--mc-fs-body);
  outline: none;
  cursor: pointer;
}

.adm__select:focus {
  border-color: var(--td-brand-color);
}

.adm__table {
  width: 100%;
  border-collapse: collapse;
  font-size: var(--mc-fs-body);
}

.adm__table th {
  text-align: left;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  padding: 8px;
  border-bottom: 1px solid var(--mc-hairline);
}

.adm__table td {
  padding: 8px;
  border-bottom: 1px solid var(--mc-hairline);
  vertical-align: top;
}

.adm__username {
  font-weight: 500;
}

.adm__uname {
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-placeholder);
}

.adm__reason {
  margin-top: 2px;
  font-size: var(--mc-fs-sm);
  color: var(--td-error-color);
}

.adm__muted {
  color: var(--td-text-color-secondary);
  font-size: var(--mc-fs-sm);
}

.adm__until {
  display: block;
  margin-top: 2px;
  font-size: var(--mc-fs-caption);
  color: var(--td-text-color-placeholder);
}

.adm__ops {
  text-align: right;
  white-space: nowrap;
}

.adm__ops-col {
  width: 150px;
}

.adm__empty {
  text-align: center;
  color: var(--td-text-color-secondary);
  padding: 24px 8px;
}

.adm__pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  font-size: var(--mc-fs-sm);
}

.adm__page {
  margin: 0 8px;
  font-variant-numeric: tabular-nums;
  color: var(--td-text-color-secondary);
}

.adm__banform {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.adm__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.adm__resetmsg {
  margin: 0 0 8px;
  font-size: var(--mc-fs-body);
}

.adm__pwd {
  padding: 12px;
  border-radius: var(--mc-radius-md);
  background: var(--td-bg-color-page);
  border: 1px solid var(--mc-hairline);
  font-family: ui-monospace, Consolas, monospace;
  font-size: var(--mc-fs-emphasis);
  letter-spacing: 0.5px;
  user-select: all;
  word-break: break-all;
}
</style>
