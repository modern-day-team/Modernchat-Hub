/** 与后端 model.User 的 JSON 形状一致（camelCase；PasswordHash/TokenVersion 不下发） */
export interface User {
  id: number
  username: string
  nickname: string
  avatarColor: string
  email?: string
  role: 'user' | 'admin'
  bannedUntil?: string
  banReason?: string
  createdAt: string
}

/** 与后端 model.ServiceInstance 的 JSON 形状一致（TokenCiphertext 不下发，只见 hasToken） */
export interface ServiceInstance {
  id: number
  name: string
  baseUrl: string
  authType: 'none' | 'bearer'
  /** 是否已配置访问凭据（凭据本体永不下发） */
  hasToken: boolean
  healthPath: string
  enabled: boolean
  lastCheckAt?: string
  lastStatus: 'ok' | 'fail' | 'unknown'
  lastVersion?: string
  note?: string
  createdAt: string
}
