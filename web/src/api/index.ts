import { http } from './client'
import type { ProbeAllResultSummary, ServiceInstance, UpdateCheck, UpdateStatus, User } from '@/types'

/** 服务实例表单载荷（camelCase，与后端 handler 契约一致；token 空值=保持原凭据） */
export interface ServiceUpsert {
  kind?: string
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

/** 服务实例端点（RESTful：/api/v1/services，探测走子资源 actions/check，更新走 update 子资源） */
export const servicesApi = {
  list: () => http.get<ServiceInstance[]>('/services'),
  create: (body: ServiceUpsert) => http.post<ServiceInstance>('/services', body),
  update: (id: number, body: ServiceUpsert) => http.put<ServiceInstance>(`/services/${id}`, body),
  remove: (id: number) => http.delete<void>(`/services/${id}`),
  check: (id: number) => http.post<ServiceInstance>(`/services/${id}/actions/check`),
  /** 批量并发探测（集合级操作；概览页一键刷新） */
  checkAll: () => http.post<ProbeAllResultSummary>('/services/actions/check-all'),
  /** 更新编排（经 kind 对应 adapter 代理目标服务） */
  updateCheck: (id: number) => http.get<UpdateCheck>(`/services/${id}/update/check`),
  updateStatus: (id: number) => http.get<UpdateStatus>(`/services/${id}/update/status`),
  updateStart: (id: number) => http.post<{ started: boolean }>(`/services/${id}/update/start`),
}
