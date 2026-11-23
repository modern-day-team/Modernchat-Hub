# Changelog

## 未发布

- feat(adapter)：`Adminer` 窄接口扩展插件、审计、公告能力——`AdminPluginPage` / `AdminAuditPage` / `AnnounceResult` 归一化结构；`modernchatnew.go` 代理主仓库 `GET /api/admin/plugins`、`POST /api/admin/plugins/global-disable`、`GET /api/admin/audits`、`POST /api/admin/announce`（空集归一为 `[]`，错误沿用 `decodeResponse` 语义映射）
- feat(server)：四项管理编排进 `InstanceService`（复用 `adminContext` 取实例→解析 adapter→解密凭据；日志只记实例名与动作，公告正文不进日志）；REST 端点 `GET /api/v1/services/:id/admin/{plugins,audits}`、`POST .../admin/plugins/global-disable`、`POST .../admin/announce`（公告正文 1–1000 字校验）
- feat(adapter/server/web)：站点设置代理——`Adminer.Settings/SaveSettings` 用 map 透传七组配置（内核不枚举目标服务字段），`GET/PUT /api/v1/services/:id/admin/settings`（PUT 要求非空 JSON 对象）；运营对话框新增「站点设置」页签，以 JSON 编辑并只提交需改动的分组，敏感字段 `hasXxx` 提示且明文密钥不会被回传或记日志
- 验证：`go build` / `go vet` / `gofmt` / `go test ./...` 全绿；`npm run build` 通过

## 0.5.0（2026-11-21）

- feat(server)：认证安全加固（router 遗留 TODO 清零）——**IP 级限流**：内存令牌桶中间件（`golang.org/x/time/rate`），注册/登录每 IP 每路由 10 次/分，超限 429「操作过于频繁，请稍后再试」；惰性清理防内存膨胀
- feat(server)：**登录失败锁定**——`login_attempts` 表按 username 计（用户不存在也计数，堵「用户名不存在」绕过；错误文案恒定不泄露账号存在性）；`ON DUPLICATE KEY UPDATE` 原子自增，达阈值写 `locked_until` 并清零计数（429 带剩余分钟），凭据正确即删除记录，锁定期过后自动失效；锁定检查先于密码校验；计数失败只记日志降级放行（不阻塞主流程）
- feat(server)：`TRUSTED_PROXIES` 显式配置（默认仅回环；显式空串 = 不信任任何代理）——ClientIP 防 X-Forwarded-For 伪造，限流与审计依赖其正确性
- config：`LOGIN_MAX_ATTEMPTS`（0=关闭，否则 2–20，默认 5）、`LOGIN_LOCK_MINUTES`（1–1440，默认 15）、`TRUSTED_PROXIES`
- 端到端验证：3 次错密码即触发锁定（429 带剩余分钟）→ 锁定期内再试仍 429（检查先行于密码校验）→ 1 分钟自动过期后正确密码 200；注册端点连打 11 次 = 400×10 + 429×1，且登录端点配额独立不受影响
- 版本同步：`web/package.json` 0.5.0

## 0.4.0（2026-11-21）

- feat(adapter)：统一管理后台落地——`adapter.Adminer` 窄接口（Stats/Users/Ban/Unban/Kick/ResetPassword）与 `Registry` 双能力注册（更新 + 管理）；`modernchatnew.go` 代理主仓库站长 API `/api/admin/{stats,users,users/:id/{ban,unban,kick,reset-password}}`（响应形状逐字段对齐主仓库需求规格 §3.2，指针字段 nil 归一）
- feat(server)：管理操作编排进 `InstanceService`（解析 adminer → 解密凭据 → 代理调用，日志只记实例名/动作/目标 id）；REST 端点 `GET /api/v1/services/:id/admin/{stats,users}`、`POST .../admin/users/:uid/{ban,unban,kick,reset-password}`；分页与筛选参数归一（page≥1、pageSize 1–100、kw≤50、role/status 枚举收敛、days 0–3650）；重置密码的一次性明文只在响应出现，不落库不进日志
- feat(web)：服务行新增「管理」入口（kind=modernchat-new）——远程管理对话框：概览统计 8 卡、用户表（搜索/角色/状态筛选 + 分页）、封禁（天数 0=永久 + 原因）/解封/强制下线/重置密码（一次性明文提示只显示一次）
- fix(server)：`ErrKindUnknown` 文案通用化为「该服务类型不支持此操作」（更新与管理两域共用）；`server/.env.example` 补 `GIN_MODE`
- 端到端验证（node 假主仓库）：统计归一（conversations=15=10+5）、用户行指针字段归一、封禁体 `{days,reason}` 到达目标、kick `wasOnline`、重置密码明文回传、generic 优雅降级 400
- 版本同步：`web/package.json` 0.4.0

## 0.3.0（2026-11-21）

- perf(server)：GORM 连接池对齐主仓库（MaxOpen 200 / MaxIdle 50 / ConnMaxLifetime 1h，兜底突发并避免坏连接）；`TranslateError` 把唯一键冲突转为 `gorm.ErrDuplicatedKey`（注册/实例重名并发兜底映射 409）；gin 模式由 `GIN_MODE` 控制且**默认 release**（去 debug 告警与路由打印）；HTTP server 补 `IdleTimeout 90s` + `ReadHeaderTimeout 10s`（项目组口径：防 Slowloris、回收 keep-alive，不设整体读写超时以免误杀长连接）
- perf(server)：REST 响应 gzip（`gin-contrib/gzip`，DefaultCompression、`MinLength=1024`、预留 `/ws` 排除口径——未来 WS 握手依赖 Hijack 不能被 gzip 包装）
- feat(server)：批量并发探测 `POST /api/v1/services/actions/check-all`——有界并发（上限 8，手写信号量不引新依赖），单实例探测失败只落 fail 不影响其他；概览页一键刷新全部实例状态
- perf(web)：构建产物预压缩 `.gz`/`.br`（移植项目组 `scripts/precompress.mjs`，Node 内置 zlib 零依赖；配合 nginx `gzip_static/brotli_static` 零运行时 CPU）——实测 brotli 后约为原始体积 29%
- fix(server)：字段长度校验补齐（BaseURL≤255 / HealthPath≤100 / Note≤255 字符，与列宽对齐）——此前超长 note 落库时报 1406 → 500，现在统一 400
- feat(web)：概览页「全部探测」按钮（完成提示「{ok} 健康 / {fail} 异常」）
- 端到端验证：批量探测 4/4 ok、gzip 2157B→287B（-87%）、release 模式无 debug 日志、超长字段 400
- 版本同步：`web/package.json` 0.3.0

## 0.2.0（2026-11-21）

- feat(adapter)：更新 adapter 落地——`server/internal/adapter/` 定义 `Updater` 窄接口（Check/Status/Start）与 `Registry`（组合根唯一装配点），内置 `modernchatnew.go` 对接主仓库 `/api/admin/update/{status,check,start}`（15s 超时）；目标响应经 `decodeResponse` 统一归一：401/403→凭据无效、404→接口不存在、409→任务进行中、400→部署不支持（目标原因透传）、网络失败→不可达
- feat(server)：`ServiceInstance` 新增 `kind`（纯数据，空值归一 generic，内核不枚举项目组服务类型）；更新编排进 `InstanceService`（解析 adapter → 解密凭据 → 代理调用），REST 端点 `GET /api/v1/services/:id/update/{check,status}`、`POST .../update/start`；未知 kind 一律 400 `该服务类型不支持更新操作`
- feat(web)：服务表单新增「服务类型」选择；`kind=modernchat-new` 的实例行内出现「更新」入口——更新对话框支持状态轮询（3s）、检查新版本（当前→最新→有/无更新）、开始更新（busy 时禁用）
- 单测：adapter 目标响应归一七种情形（httptest 假目标）+ 编排三支（凭据解密送达/未知 kind/错误透传）；端到端验证 generic 降级、目标缺接口、假目标全链路（status/check/start）、凭据错误 403
- 版本同步：`web/package.json` 0.2.0

## 0.1.0（2026-11-21）

- 仓库定位确立：Modernchat 项目组通用型统一管理面板（统一管理后台 + 统一更新管理），核心域模型为「服务实例」，项目组特化逻辑经 adapter 窄接口注入内核，不硬编码任何特定服务
- feat(server)：项目组同构骨架（Go 1.26 + Gin + GORM + MySQL 8，:8091）——账号体系（JWT HS256 claims uid+tv、mc_auth cookie 双通道、登出 bump token_version、首个注册者=admin）+ 通用服务实例注册/编辑/删除与健康·版本探测（5s 超时，探测结果作为数据写回 lastStatus/lastVersion）；凭据 nacl/secretbox 加密落库，密钥经独立前缀 mc-hub-cred-v1: 派生；单测覆盖 token/crypto/实例业务（httptest 假目标，不依赖 MySQL）
- feat(web)：Vue 3 + Vite + TS + Pinia + UnoCSS 管理面板（:5176，/api 代理 :8091）——登录/注册（首个注册者=admin 提示）、概览统计（健康/异常/未探测）、服务管理（列表 + 新增/编辑弹窗 + 行内「检查」探测 + 删除确认）；Ui* 控件层（UiButton/UiInput/UiCheckbox/UiTag/UiDialog）；令牌色、禁 v-html、zh-CN/en-US 双语
- 仓库约定文件：`.gitattributes` 强制 LF（项目组约定）；`.gitignore` 覆盖 Go 产物与前端依赖/构建产物，AGENTS.md 仅留本地不入库
- 本地开发环境：docker-compose MySQL 8（映射 3308，避开主仓库 3306 与 SSO 3307），库 modernchat_hub，卷 hub-mysql-data，healthcheck 与 SSO 同款
- CI：GitHub Actions 双端门禁（server gofmt/vet/build/test -race + 挂 MySQL 8 服务容器；web npm ci → type-check → build → oxlint/eslint 只检查不改文件），push 与 PR 触发
- `server/.env.example`：PORT 8091、DB_DSN 指向 3308（含本机原生 MySQL 3306 备选写法）、JWT_SECRET 占位（≥32 字节）、COOKIE_SECURE、CRED_SECRET（可选，缺省由 JWT_SECRET 派生 mc-hub-cred-v1: 前缀）
- README 中文文档：定位、技术栈、快速开始、端口表（server 8091 / web 5176 / MySQL 3308）、目录结构树、API 契约表
