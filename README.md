# Unified-Console

Modernchat 项目组的通用型统一管理面板（Modernchat-Hub）：把项目组各服务的站长后台收敛到一处——**统一管理后台 + 统一更新管理**。核心域模型是「服务实例」：名称、base URL、鉴权方式、健康检查端点全部数据化配置，内核不硬编码任何特定服务的逻辑；新增一个被管理服务 = 注册配置 +（必要时）一个 adapter，不改内核代码。

## 技术栈

| 端 | 技术 |
|---|---|
| 后端 `server/` | Go 1.26 + Gin + GORM（MySQL 8） |
| 前端 `web/` | Vue 3 + Vite + TypeScript + Pinia + Vue Router + UnoCSS 原子类 |

## 核心能力

- **统一管理后台**：管理员在 Hub 一处完成项目组各站点的用户管理、站点设置、公告、插件上下架、审计等，经各服务的管理 API 远程操作；各服务管理员凭据由 Hub 加密落库托管
- **统一更新管理（0.2.0 已落地）**：对 `kind=modernchat-new` 的实例，经 adapter 代理主仓库在线更新——检查新版本、轮询更新任务状态、远程触发更新；目标 `WEB_UPDATE=false` 或部署方式不支持时优雅降级（原因透传）；新增项目组服务类型 = 新 adapter + 组合根一行注册
- **通用内核**：服务注册/连接管理、健康与状态聚合、凭据托管、管理操作代理、版本与更新编排；项目组特化逻辑放独立 adapter 包（`server/internal/adapter/`），经窄接口注入内核

## 快速开始

```bash
# 1. 起本地 MySQL（映射 3308，避开主仓库 3306 与 SSO 3307）
docker compose up -d

# 2. 配置环境变量
cd server && cp .env.example .env   # 填入 JWT_SECRET

# 3. 运行后端（启动期 AutoMigrate 自动建表）
go run ./cmd/server

# 4. 前端（另开终端）
cd ../web && npm install && npm run dev
```

## 端口

| 服务 | 端口 | 说明 |
|---|---|---|
| server | 8091 | 后端 API（`/api/v1` 前缀，RESTful） |
| web | 5176 | 前端 dev server，`/api` 代理到 :8091 |
| MySQL | 3308 | 主仓库占 3306、SSO 占 3307，故用 3308 |

## 目录结构

```
server/
  cmd/server/          # 入口（开发运行，启动期 AutoMigrate）
  internal/
    config/            # 配置加载（.env 可选，JWT_SECRET 必填 ≥32 字节）
    version/           # 构建版本信息（/api/version，ldflags 注入）
    token/             # JWT（叶子包，项目组口径 uid+tv、TTL 7 天）
    apperr/            # 哨兵错误（叶子包）
    crypto/            # 服务凭据加解密（nacl/secretbox，独立派生前缀 mc-hub-cred-v1:）
    model/             # GORM 模型（User / ServiceInstance）
    store/             # 数据访问
    service/           # 业务规则唯一归属（账号 / 服务实例与探测）
    handler/           # 传输层零业务（绑定/组装/状态码映射）
    middleware/        # 认证 / 管理员角色校验
    adapter/           # 项目组特化适配（Updater 窄接口 + Registry；modernchat-new 已接入）
    router/            # 唯一组合根
web/
  src/
    api/               # API client（BASE /api/v1，错误归一 ApiError）
    assets/            # 令牌体系 main.css（--td-*/--mc-*）
    components/ui/     # 自研 Ui* 控件层
    i18n/              # zh-CN / en-US 键同步
    router/
    stores/            # Pinia
    types/
    views/
```

## API（v1，RESTful）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 探活（免鉴权） |
| GET | `/api/version` | 版本 `{version,commit,buildTime}`（免鉴权） |
| POST | `/api/v1/users` | 注册（首个注册者自动成为 admin） |
| POST | `/api/v1/sessions` | 登录，返回 `{token,user}` 并种 `mc_auth` cookie |
| GET | `/api/v1/users/me` | 当前用户（需认证） |
| DELETE | `/api/v1/sessions/current` | 登出（token_version+1，全部旧 token 失效） |
| GET / POST | `/api/v1/services` | 服务实例列表 / 注册（需管理员；`kind` 为服务类型） |
| GET / PUT / DELETE | `/api/v1/services/:id` | 详情 / 编辑（token 空值=保持原凭据）/ 删除 |
| POST | `/api/v1/services/:id/actions/check` | 立即探测健康与版本 |
| POST | `/api/v1/services/actions/check-all` | 批量并发探测全部实例（有界并发 8；概览页一键刷新） |
| GET | `/api/v1/services/:id/update/check` | 检查目标服务新版本（经 kind 对应 adapter） |
| GET | `/api/v1/services/:id/update/status` | 目标更新任务状态快照（轮询） |
| POST | `/api/v1/services/:id/update/start` | 触发目标在线更新（异步） |
| GET | `/api/v1/services/:id/admin/stats` | 目标服务概览统计（统一管理后台，经 adapter 代理） |
| GET | `/api/v1/services/:id/admin/users` | 目标用户分页列表（page/pageSize/kw/role/status） |
| POST | `/api/v1/services/:id/admin/users/:uid/ban` | 封禁目标用户（`{days:0-3650, reason?}`，0=永久） |
| POST | `/api/v1/services/:id/admin/users/:uid/unban` | 解封 |
| POST | `/api/v1/services/:id/admin/users/:uid/kick` | 强制下线（不封禁，返回 wasOnline） |
| POST | `/api/v1/services/:id/admin/users/:uid/reset-password` | 目标端随机重置密码（一次性明文） |

## 约定

- 开发约定见仓库内 `AGENTS.md`（本地文件，不入库）
- 文档、注释、UI 文案、提交描述一律中文；提交信息 Conventional Commits（`feat: xxx` / `fix: xxx`）
- 仓库强制 LF（`.gitattributes`）
- **严禁把凭据写入仓库**（项目组历史教训）；`.env` 已 gitignore
