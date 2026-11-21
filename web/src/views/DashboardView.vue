<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import UiTag from '@/components/ui/UiTag.vue'
import { useInstancesStore } from '@/stores/instances'
import type { ServiceInstance } from '@/types'

const { t } = useI18n()
const instances = useInstancesStore()
const loaded = ref(false)

onMounted(async () => {
  await instances.fetchAll()
  loaded.value = true
})

const counts = computed(() => {
  const total = instances.list.length
  const ok = instances.list.filter((i) => i.lastStatus === 'ok').length
  const fail = instances.list.filter((i) => i.lastStatus === 'fail').length
  return { total, ok, fail, unknown: total - ok - fail }
})

function statusTheme(s: ServiceInstance['lastStatus']): 'success' | 'danger' | 'default' {
  return s === 'ok' ? 'success' : s === 'fail' ? 'danger' : 'default'
}

function statusText(s: ServiceInstance['lastStatus']) {
  return s === 'ok' ? t('svc.statusOk') : s === 'fail' ? t('svc.statusFail') : t('svc.statusUnknown')
}

function fmtTime(iso?: string) {
  return iso ? new Date(iso).toLocaleString() : t('dash.none')
}
</script>

<template>
  <div>
    <h1 class="dash__title">{{ t('dash.title') }}</h1>

    <!-- 状态统计卡 -->
    <div class="dash__cards">
      <div class="dash__card">
        <div class="dash__num">{{ counts.total }}</div>
        <div class="dash__label">{{ t('dash.total') }}</div>
      </div>
      <div class="dash__card">
        <div class="dash__num dash__num--ok">{{ counts.ok }}</div>
        <div class="dash__label">{{ t('dash.healthy') }}</div>
      </div>
      <div class="dash__card">
        <div class="dash__num dash__num--fail">{{ counts.fail }}</div>
        <div class="dash__label">{{ t('dash.abnormal') }}</div>
      </div>
      <div class="dash__card">
        <div class="dash__num">{{ counts.unknown }}</div>
        <div class="dash__label">{{ t('dash.unknown') }}</div>
      </div>
    </div>

    <!-- 服务状态简表 -->
    <div class="dash__panel ui-panel">
      <div v-if="loaded && instances.list.length === 0" class="dash__empty">
        <p>{{ t('dash.noServices') }}</p>
        <router-link to="/services" class="dash__go">{{ t('dash.goAdd') }}</router-link>
      </div>
      <table v-else class="dash__table">
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
</template>

<style scoped>
.dash__title {
  margin: 0 0 16px;
  font-size: var(--mc-fs-title);
  font-weight: 600;
}

.dash__cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.dash__card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--mc-hairline);
  border-radius: var(--mc-radius-lg);
  padding: 16px 20px;
}

.dash__num {
  font-size: var(--mc-fs-display);
  font-weight: 600;
  line-height: 1.2;
}

.dash__num--ok {
  color: var(--td-success-color);
}

.dash__num--fail {
  color: var(--td-error-color);
}

.dash__label {
  margin-top: 4px;
  font-size: var(--mc-fs-sm);
  color: var(--td-text-color-secondary);
}

.dash__panel {
  padding: 8px 16px;
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
  padding: 10px 8px;
  border-bottom: 1px solid var(--mc-hairline);
}

.dash__table td {
  padding: 10px 8px;
  border-bottom: 1px solid var(--mc-hairline);
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
  padding: 32px 8px;
  text-align: center;
  color: var(--td-text-color-secondary);
}

.dash__go {
  display: inline-block;
  margin-top: 8px;
  font-size: var(--mc-fs-sm);
}
</style>
