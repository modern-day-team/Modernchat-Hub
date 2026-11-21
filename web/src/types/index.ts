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
  /** 服务类型：generic=仅健康/版本探测；其余取值由后端注册的 adapter 决定 */
  kind: string
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

/** 更新检查结果（adapter 归一后的跨服务口径） */
export interface UpdateCheck {
  currentVersion: string
  latestVersion: string
  updateAvailable: boolean
}

/** 目标服务更新任务的运行状态快照 */
export interface UpdateStatus {
  available: boolean
  busy: boolean
  phase: string
  lines?: string[]
  error?: string
  currentVersion?: string
  mode?: string
}

/** 批量探测的单实例结果 */
export interface ProbeAllResult {
  id: number
  name: string
  lastStatus: 'ok' | 'fail'
  lastVersion?: string
}

/** 批量探测汇总（后端有界并发，单实例失败不影响其他） */
export interface ProbeAllResultSummary {
  total: number
  ok: number
  fail: number
  results: ProbeAllResult[]
}
