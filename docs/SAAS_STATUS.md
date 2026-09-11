# SaaS 实施状态

目标和产品决策以 [SAAS_SPEC.md](SAAS_SPEC.md) 为准。**禁止偏离。**

## 架构硬约束（不可改）
- 同一套部署、同一进程（可水平多副本），共享同一数据库
- `/t/{slug}` 中间件注入 `tenant_id`；数据层强制 scope；缓存/会话隔离
- **禁止**每租户独立工作进程 / supervisor / Bound worker
- 平台与空间双层账号；设置按租户；包月限额；人工开通套餐；少改 relay/渠道

## 当前状态

实现和本机验收已完成，正在提交到 `feat/saas-multitenant`，随后创建到 `Sunwuyuan/new-api-sass:main` 的 PR。没有单租户模式开关。

## 已交付

- 独立 `tenant/`、`platform/`、`plan/` 包；平台账号、服务器端会话、空间、套餐、月用量、人工套餐记录和一次性 root 激活模型。
- `/platform` 平台控制台：注册、登录、改密、创建空间、用量/套餐查看、管理员用户与空间列表、人工开通套餐、停用与恢复。创建空间后交付 30 分钟有效、一次性使用的 root 激活链接；平台与空间密码独立。
- 单个共享网关通过 `/t/:slug/*path` 解析租户。业务表和独立日志库强制 tenant scope；缺上下文拒绝，客户端不能注入或改写 `tenant_id`；禁止运行期原始业务 SQL，阻止跨租户 ID、更新、删除与 upsert。
- 设置、SMTP、OAuth 注册表、渠道/权限/价格/亲和缓存和插件运行状态按租户保存。设置以不可变快照发布；Redis 强制租户命名空间，磁盘文件/制品与异步任务锁包含租户维度。
- Cookie path、JWT/refresh 签名、Web Storage、跨标签通知按租户隔离。一个共享调度器执行带租户上下文的后台任务；缺上下文的任务轮询拒绝执行。
- Lite：UTC 自然月 1,000 次请求、5 个用户（含 root）、20 个令牌、3 个渠道，强制平台页脚。Pro：100,000 次请求、1,000 个用户、10,000 个令牌、100 个渠道，允许修改平台页脚。人工开通 1–36 个月；到期或停用拒绝访问。
- 月额度通过数据库条件更新原子预留；成功或计费请求保留计数，未收费失败释放预留。达到上限返回 HTTP 429 / `tenant_monthly_limit_exceeded`，升级后立即恢复。用户/令牌/渠道创建在事务中检查资源限额。
- 保留 New API 原有渠道协议和上游计费，只传播租户上下文及计量标记。新增渠道立即刷新当前租户路由缓存；前端/静态资源兜底拒绝非 GET/HEAD 请求，防止匿名 POST 消耗网关月额度。
- 复用原 New API 空间 UI 和共享 `Dialog`、`ConfirmDialog`、`CopyButton`、`PasswordInput`、`Field`、加载/错误状态组件；新增界面文案覆盖 en、zh、zh-TW、fr、ja、ru、vi。
- [中文 README](../README.zh_CN.md) 已提供随机密钥生成、首次管理员、启动、人工套餐、旧库导入和多副本说明。Compose 构建当前分支，凭据读取本地 `.env`，使用 `/platform/api/plans` GET 健康检查；构建上下文排除密钥与本地数据。

## 真实数据库验收

日期：2026-09-11。工具：Go 1.25.1、Bun 1.4.2、Chromium 153.0.8010.12、Docker Compose 2.39.4（配置校验）。

| 数据库 | 精确版本 / 本机端口 | 新库 + 第二次启动 | 发布版旧库升级 + 第二次启动 | SaaS 契约测试 |
| --- | --- | --- | --- | --- |
| SQLite | Go 驱动 SQLite 3.50.4 | 通过 | 通过 | 通过 |
| MySQL | 8.4.11 / 33306 | 通过 | 通过 | 通过 |
| PostgreSQL | 17.11（Debian 17.11-0+deb13u1）/ 35432 | 通过 | 通过 | 通过 |
| 独立 ClickHouse 日志库（SQLite 主库） | 26.3.33.24-lts / native 39000、HTTP 38123 | 通过 | 通过 | 通过 |
| Redis | 8.0.2 / 33379 | 真实连接通过 | 不涉及模式迁移 | 命名空间、脚本和无上下文拒绝通过 |

旧库由最新上游发布版 **v1.0.0-rc.37** 实际启动建立，录入 root、令牌、渠道、能力、站点设置、消费和审计记录。四套旧服务完全停止后再启动 SaaS；对 users/tokens/channels/abilities/指定 options 的内容摘要进行前后比较。消费额度 21、既有审计记录、用户名/令牌唯一性和业务数据保留；旧 root 可在 `/t/imported/` 登录，平台归属及空间计数正确，ServerAddress 更新到新路径。迁移完成后没有残留复制用备份表，第二次启动不重复导入。

MySQL/PostgreSQL 都覆盖独立关系型日志库，ClickHouse 覆盖日志及审计表加列、租户读取和清理隔离。令牌迁移另覆盖旧 48 字符列、跨租户同 key、同租户唯一性、PostgreSQL 旧 UNIQUE 约束，以及未知/可延迟约束的拒绝与保留。

实际命令（测试数据库为空的本机临时库，不是生产凭据）：

```bash
# SQLite + 主流程
go test . -run TestSaaSContracts -count=1

# MySQL + 独立 MySQL 日志库
SAAS_TEST_DSN='root@tcp(127.0.0.1:33306)/saas_contract_final?charset=utf8mb4&parseTime=true' \
SAAS_TEST_LOG_DSN='root@tcp(127.0.0.1:33306)/saas_contract_final_log?charset=utf8mb4&parseTime=true' \
go test . -run TestSaaSContracts -count=1

# PostgreSQL + 独立 PostgreSQL 日志库
SAAS_TEST_DSN='postgres://box@127.0.0.1:35432/saas_contract_final?sslmode=disable' \
SAAS_TEST_LOG_DSN='postgres://box@127.0.0.1:35432/saas_contract_final_log?sslmode=disable' \
go test . -run TestSaaSContracts -count=1

# SQLite + 独立 ClickHouse 日志库
SAAS_TEST_LOG_DSN='clickhouse://default@127.0.0.1:39000/saas_contract_final_log' \
go test . -run TestSaaSContracts -count=1

# 旧令牌索引与约束迁移
TEST_MYSQL_DSN='root@tcp(127.0.0.1:33306)/saas_token_migration?charset=utf8mb4&parseTime=true' \
TEST_POSTGRES_DSN='postgres://box@127.0.0.1:35432/saas_token_migration?sslmode=disable' \
go test ./model -run 'TestMigrateTokenTenantScope|TestTokenTenantMigrationPostgreSQLConstraints' -count=1

SAAS_TEST_REDIS_ADDR=127.0.0.1:33379 go test . -run TestSaaSRedisIsolation -count=1
```

本机进程验收脚本/输出留在 `/tmp`：`saas-fresh-matrix.py` 对四引擎各启动两次；`saas-run-matrix.py` 在 33020–33023 启动发布版升级实例；`saas-db-check.py verify`、`saas-verify-upgrade.py` 检查摘要、日志、导入归属和登录。结果为 `/tmp/saas-fresh-matrix-results.log`、`/tmp/saas-upgrade-v3-*-{1,2}.log`；随机凭据保存在权限 0600 的临时文件，不入仓库。

## HTTP 与浏览器验收

真实 Go 服务运行在 33030，使用 SQLite 与 Redis；本机 HTTP 模拟上游在 33999，请求经过完整网关、认证、渠道选择、上游计费与持久化链路。

1. 平台浏览器注册 → 创建 Browser Alpha / Beta → 一次性激活 → 两个空间分别以加密密码登录 root；同一浏览器中的平台和两个空间会话在刷新后仍独立。
2. 分别创建渠道、API 令牌、分配钱包额度并实际转发；跨租户探测渠道/令牌/用户 ID 返回 404，借用另一租户 JWT 或 API Key 返回 401，设置与消费日志互不可见。
3. Lite 页脚写入返回 403，浏览器强制显示平台页脚。人工开通 Pro 后允许自定义页脚；Beta 的 Lite 能力保持不变，New API 项目归属保留。
4. 正常转发精确增加一次用量；上游未收费 400 不增加计数。把 Alpha 当月计数设为 1,000 后得到 429，Beta 继续可用；浏览器人工开通 Pro 后 Alpha 真实转发恢复。
5. 浏览器停用 Alpha 后其接口返回 403，Beta 正常；恢复后 Alpha 可用。平台改密验证旧密码并撤销其他浏览器会话，新密码可重新登录。
6. 未注册路径和静态资源的匿名 POST 返回 404，不消耗月额度；重启后重复确认正常转发与计量。

浏览器脚本位于 `/tmp/saas-browser/`，HTTP 结果位于 `/tmp/saas-live-http-results-2.log`、`/tmp/saas-live-upgrade-result.log`。可审查截图：[平台控制台](../.github/assets/saas-platform.png)、[人工套餐管理](../.github/assets/saas-plan-administration.png)。

## 构建与回归

以下已通过：

```bash
go build -o /tmp/new-api-saas .
go vet ./...
go test ./...
go test -race . -run TestSaaSContracts -count=1
# 最后两处修复分别复跑根包/router 和 controller 测试
go test . ./router -count=1
go test ./controller
cd relaykit
GOWORK=off go build ./...
GOWORK=off go vet ./...
GOWORK=off go test ./...
cd ../web
bun run test --maxWorkers=2 --testTimeout=15000
bun run build:check
bun run typecheck
```

前端全量 **135 个测试文件 / 1,478 项测试通过**；涉及的 64 个 TS/TSX 文件 lint 无 error，71 个前端修改文件通过保留版权头的格式检查。生产构建成功，分包总代码约 58.9 MB（gzip 17.1 MB，含按需加载资源）。`git diff --check` 通过，修改文件与本机随机凭据比对无泄漏，既有项目版权头文本保持一致。

额外执行的全仓 `bun run copyright:check` 退出 1，报告 11 处既有文件头格式/缺失；已在起始 HEAD `385d2dfd1` 独立复测，报告文件完全相同，本次没有新增该类问题。

## 安全依据与运行边界

实现前检查了 OWASP [Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)、[CSRF Prevention](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html) 指南及 **ASVS 5.0.0** 的认证/会话要求（6.2.1–6.2.8、7.2.1–7.2.4、7.4.1）。平台使用 Argon2id、服务器端散列会话、8 小时绝对/30 分钟空闲过期、来源与会话绑定 CSRF 校验、数据库限流；新密码阻止常见泄漏密码。回归覆盖跨租户凭据、缺失/错误 CSRF、过期/重放激活、错误旧密码、改密撤销和会话失效；审计不写密码、激活原文和可用令牌。以上是具体控制及验收记录，不代表对整个上游项目的全面 ASVS 认证。

- 本环境没有 Docker daemon；Compose 2.39.4 的 `config --quiet` 已通过，实际启动验收使用 README 中的 Go 等价方式，没有将容器启动声称为已实测。
- 旧库升级需停旧服务、备份并单实例迁移。未知自定义唯一索引/约束会拒绝自动迁移，须显式补齐租户维度。验证范围是上表实际版本。
- 公开部署使用实际 HTTPS `PLATFORM_ORIGIN`；多个副本共用数据库、Redis、密钥和文件/对象存储，设置和缓存按共享调度周期同步。原有外部 OAuth/支付回调需改到空间路径。
