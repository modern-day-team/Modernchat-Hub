# Changelog

## 0.3.0（2026-11-21）

- perf(server)：GORM 连接池对齐主仓库（MaxOpen 200 / MaxIdle 50 / ConnMaxLifetime 1h，兜底突发并避免坏连接）；`TranslateError` 把唯一键冲突转为 `gorm.ErrDuplicatedKey`（注册/实例重名并发兜底映射 409）；gin 模式由 `GIN_MODE` 控制且**默认 release**（去 debug 告警与路由打印）；HTTP server 补 `IdleTimeout 90s` + `ReadHeaderTimeout 10s`（家族口径：防 Slowloris、回收 keep-alive，不设整体读写超时以免误杀长连接）
- perf(server)：REST 响应 gzip（`gin-contrib/gzip`，DefaultCompression、`MinLength=1024`、预留 `/ws` 排除口径——未来 WS 握手依赖 Hijack 不能被 gzip 包装）
- feat(server)：批量并发探测 `POST /api/v1/services/actions/check-all`——有界并发（上限 8，手写信号量不引新依赖），单实例探测失败只落 fail 不影响其他；概览页一键刷新全部实例状态
- perf(web)：构建产物预压缩 `.gz`/`.br`（移植家族 `scripts/precompress.mjs`，Node 内置 zlib 零依赖；配合 nginx `gzip_static/brotli_static` 零运行时 CPU）——实测 brotli 后约为原始体积 29%
- fix(server)：字段长度校验补齐（BaseURL≤255 / HealthPath≤100 / Note≤255 字符，与列宽对齐）——此前超长 note 落库时报 1406 → 500，现在统一 400
- feat(web)：概览页「全部探测」按钮（完成提示「{ok} 健康 / {fail} 异常」）
- 端到端验证：批量探测 4/4 ok、gzip 2157B→287B（-87%）、release 模式无 debug 日志、超长字段 400
- 版本同步：`web/package.json` 0.3.0

## 0.2.0（2026-11-21）

- feat(adapter)：更新 adapter 落地——`server/internal/adapter/` 定义 `Updater` 窄接口（Check/Status/Start）与 `Registry`（组合根唯一装配点），内置 `modernchatnew.go` 对接主仓库 `/api/admin/update/{status,check,start}`（15s 超时）；目标响应经 `decodeResponse` 统一归一：401/403→凭据无效、404→接口不存在、409→任务进行中、400→部署不支持（目标原因透传）、网络失败→不可达
- feat(server)：`ServiceInstance` 新增 `kind`（纯数据，空值归一 generic，内核不枚举家族服务类型）；更新编排进 `InstanceService`（解析 adapter → 解密凭据 → 代理调用），REST 端点 `GET /api/v1/services/:id/update/{check,status}`、`POST .../update/start`；未知 kind 一律 400 `该服务类型不支持更新操作`
- feat(web)：服务表单新增「服务类型」选择；`kind=modernchat-new` 的实例行内出现「更新」入口——更新对话框支持状态轮询（3s）、检查新版本（当前→最新→有/无更新）、开始更新（busy 时禁用）
- 单测：adapter 目标响应归一七种情形（httptest 假目标）+ 编排三支（凭据解密送达/未知 kind/错误透传）；端到端验证 generic 降级、目标缺接口、假目标全链路（status/check/start）、凭据错误 403
- 版本同步：`web/package.json` 0.2.0

## 0.1.0（2026-11-21）

- 仓库定位确立：Modernchat 家族通用型统一管理面板（统一管理后台 + 统一更新管理），核心域模型为「服务实例」，家族特化逻辑经 adapter 窄接口注入内核，不硬编码任何特定服务
- feat(server)：家族同构骨架（Go 1.26 + Gin + GORM + MySQL 8，:8091）——账号体系（JWT HS256 claims uid+tv、mc_auth cookie 双通道、登出 bump token_version、首个注册者=admin）+ 通用服务实例注册/编辑/删除与健康·版本探测（5s 超时，探测结果作为数据写回 lastStatus/lastVersion）；凭据 nacl/secretbox 加密落库，密钥经独立前缀 mc-hub-cred-v1: 派生；单测覆盖 token/crypto/实例业务（httptest 假目标，不依赖 MySQL）
- feat(web)：Vue 3 + Vite + TS + Pinia + UnoCSS 管理面板（:5176，/api 代理 :8091）——登录/注册（首个注册者=admin 提示）、概览统计（健康/异常/未探测）、服务管理（列表 + 新增/编辑弹窗 + 行内「检查」探测 + 删除确认）；Ui* 控件层（UiButton/UiInput/UiCheckbox/UiTag/UiDialog）；令牌色、禁 v-html、zh-CN/en-US 双语
- 仓库约定文件：`.gitattributes` 强制 LF（家族约定）；`.gitignore` 覆盖 Go 产物与前端依赖/构建产物，AGENTS.md 仅留本地不入库
- 本地开发环境：docker-compose MySQL 8（映射 3308，避开主仓库 3306 与 SSO 3307），库 modernchat_hub，卷 hub-mysql-data，healthcheck 与 SSO 同款
- CI：GitHub Actions 双端门禁（server gofmt/vet/build/test -race + 挂 MySQL 8 服务容器；web npm ci → type-check → build → oxlint/eslint 只检查不改文件），push 与 PR 触发
- `server/.env.example`：PORT 8091、DB_DSN 指向 3308（含本机原生 MySQL 3306 备选写法）、JWT_SECRET 占位（≥32 字节）、COOKIE_SECURE、CRED_SECRET（可选，缺省由 JWT_SECRET 派生 mc-hub-cred-v1: 前缀）
- README 中文文档：定位、技术栈、快速开始、端口表（server 8091 / web 5176 / MySQL 3308）、目录结构树、API 契约表
