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
  versionPath: string
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

/** 目标服务概览统计（经 adapter 归一） */
export interface TargetAdminStats {
  usersTotal: number
  usersToday: number
  usersBanned: number
  usersAdmins: number
  conversations: number
  messagesTotal: number
  messagesToday: number
  pluginsInstalled: number
  online: number
  uptimeSeconds: number
  goVersion?: string
}

/** 目标服务用户行（adapter 归一，凭据类字段不透传） */
export interface TargetAdminUser {
  id: number
  username: string
  nickname: string
  avatarColor?: string
  email?: string
  role: 'user' | 'admin'
  bannedUntil?: string
  banReason?: string
  createdAt?: string
}

/** 目标服务用户分页 */
export interface TargetAdminUserPage {
  users: TargetAdminUser[]
  total: number
  page: number
  pageSize: number
}

/** 目标服务插件聚合行 */
export interface TargetAdminPlugin {
  name: string
  version: string
  installCount: number
  enabledCount: number
  globalDisabled: boolean
  createdAt?: string
}

export interface TargetAdminPluginPage {
  plugins: TargetAdminPlugin[]
  total: number
  page: number
  pageSize: number
}

/** 目标服务审计行 */
export interface TargetAdminAudit {
  id: number
  adminId: number
  action: string
  targetType?: string
  targetId?: number
  targetName?: string
  detail?: string
  ip?: string
  createdAt?: string
}

export interface TargetAdminAuditPage {
  audits: TargetAdminAudit[]
  total: number
  page: number
  pageSize: number
}

export interface AnnounceResult {
  ok: boolean
  convId?: number
  messageId?: number
  recipients: number
}
