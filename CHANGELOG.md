# Changelog

## 0.1.0（2026-11-21）

- 仓库定位确立：Modernchat 家族通用型统一管理面板（统一管理后台 + 统一更新管理），核心域模型为「服务实例」，家族特化逻辑经 adapter 窄接口注入内核，不硬编码任何特定服务
- feat(server)：家族同构骨架（Go 1.26 + Gin + GORM + MySQL 8，:8091）——账号体系（JWT HS256 claims uid+tv、mc_auth cookie 双通道、登出 bump token_version、首个注册者=admin）+ 通用服务实例注册/编辑/删除与健康·版本探测（5s 超时，探测结果作为数据写回 lastStatus/lastVersion）；凭据 nacl/secretbox 加密落库，密钥经独立前缀 mc-hub-cred-v1: 派生；单测覆盖 token/crypto/实例业务（httptest 假目标，不依赖 MySQL）
- feat(web)：Vue 3 + Vite + TS + Pinia + UnoCSS 管理面板（:5176，/api 代理 :8091）——登录/注册（首个注册者=admin 提示）、概览统计（健康/异常/未探测）、服务管理（列表 + 新增/编辑弹窗 + 行内「检查」探测 + 删除确认）；Ui* 控件层（UiButton/UiInput/UiCheckbox/UiTag/UiDialog）；令牌色、禁 v-html、zh-CN/en-US 双语
- 仓库约定文件：`.gitattributes` 强制 LF（家族约定）；`.gitignore` 覆盖 Go 产物与前端依赖/构建产物，AGENTS.md 仅留本地不入库
- 本地开发环境：docker-compose MySQL 8（映射 3308，避开主仓库 3306 与 SSO 3307），库 modernchat_hub，卷 hub-mysql-data，healthcheck 与 SSO 同款
- CI：GitHub Actions 双端门禁（server gofmt/vet/build/test -race + 挂 MySQL 8 服务容器；web npm ci → type-check → build → oxlint/eslint 只检查不改文件），push 与 PR 触发
- `server/.env.example`：PORT 8091、DB_DSN 指向 3308（含本机原生 MySQL 3306 备选写法）、JWT_SECRET 占位（≥32 字节）、COOKIE_SECURE、CRED_SECRET（可选，缺省由 JWT_SECRET 派生 mc-hub-cred-v1: 前缀）
- README 中文文档：定位、技术栈、快速开始、端口表（server 8091 / web 5176 / MySQL 3308）、目录结构树、API 契约表
