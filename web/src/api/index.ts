import { http } from './client'
import type { ServiceInstance, User } from '@/types'

/** 服务实例表单载荷（camelCase，与后端 handler 契约一致；token 空值=保持原凭据） */
export interface ServiceUpsert {
  name: string
  baseUrl: string
  authType: 'none' | 'bearer'
  token?: string
  healthPath?: string
  enabled?: boolean
  note?: string
}

/** 认证端点（RESTful v1：POST /users 注册、POST /sessions 登录） */
export const authApi = {
  /** 注册：后端 201 返回用户本体（不含 token），前端随后自动登录 */
  register: (body: { username: string; password: string; nickname: string; email?: string }) =>
    // 公开接口：与登录态无关，不带 Authorization、401 也不清会话
    http.post<User>('/users', body, { skipAuth: true }),
  login: (body: { username: string; password: string }) =>
    http.post<{ token: string; user: User }>('/sessions', body, { skipAuth: true }),
  me: () => http.get<User>('/users/me'),
  /** 登出：后端 token_version+1，全部旧 token 失效 */
  logout: () => http.delete<void>('/sessions/current'),
}

/** 服务实例端点（RESTful：/api/v1/services，探测走子资源 actions/check） */
export const servicesApi = {
  list: () => http.get<ServiceInstance[]>('/services'),
  create: (body: ServiceUpsert) => http.post<ServiceInstance>('/services', body),
  update: (id: number, body: ServiceUpsert) => http.put<ServiceInstance>(`/services/${id}`, body),
  remove: (id: number) => http.delete<void>(`/services/${id}`),
  check: (id: number) => http.post<ServiceInstance>(`/services/${id}/actions/check`),
}
