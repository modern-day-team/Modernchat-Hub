import { ref } from 'vue'
import { defineStore } from 'pinia'

import { servicesApi, type ServiceUpsert } from '@/api'
import type { ServiceInstance } from '@/types'

/** 服务实例集合：列表加载、增删改、单条探测结果就地替换 */
export const useInstancesStore = defineStore('instances', () => {
  const list = ref<ServiceInstance[]>([])
  const loading = ref(false)

  async function fetchAll() {
    loading.value = true
    try {
      list.value = await servicesApi.list()
    } finally {
      loading.value = false
    }
  }

  async function create(body: ServiceUpsert) {
    const inst = await servicesApi.create(body)
    await fetchAll()
    return inst
  }

  async function update(id: number, body: ServiceUpsert) {
    const inst = await servicesApi.update(id, body)
    await fetchAll()
    return inst
  }

  async function remove(id: number) {
    await servicesApi.remove(id)
    await fetchAll()
  }

  /** 立即探测：结果就地替换该行，避免整表刷新 */
  async function check(id: number) {
    const inst = await servicesApi.check(id)
    const idx = list.value.findIndex((i) => i.id === id)
    if (idx >= 0 && list.value[idx]) list.value[idx] = inst
    return inst
  }

  return { list, loading, fetchAll, create, update, remove, check }
})
