# SaaS 实施状态

目标和产品决策以 [SAAS_SPEC.md](SAAS_SPEC.md) 及 [SAAS_PHASE2_SPEC.md](SAAS_PHASE2_SPEC.md) 为准。**禁止偏离。**

## 架构硬约束（不可改）
- 同一套部署、同一进程（可水平多副本），共享同一数据库
- `/t/{slug}` 中间件注入 `tenant_id`；数据层强制 scope；缓存/会话隔离
- **禁止**每租户独立工作进程 / supervisor / Bound worker
- 平台与空间双层账号；设置按租户；包月限额；人工开通套餐；少改 relay/渠道

## 当前状态

第一、二阶段实现及本机验收已完成，交付分支为 `feat/saas-multitenant`，集中更新 [PR #1](https://github.com/Sunwuyuan/new-api-sass/pull/1)。第二阶段三数据库迁移、Go/前端全量测试、生产构建及公开演示回归均通过。演示入口为 [平台控制台](https://newapi-sass.moonrend.com/platform)。没有单租户模式开关。

## 已交付

以下为第一阶段交付与验收记录（2026-09-11）；第二阶段增量和最终验证结果见下文。

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

## 第二阶段（实现与验收完成）

目标补充以 [SAAS_PHASE2_SPEC.md](SAAS_PHASE2_SPEC.md) 为准。

### 已验收：平台数据和 API

- Lite / Standard / Pro 种子及能力迁移：保留旧 Pro ID、已有价格/限额及明确配置的能力；只补缺失能力。默认月请求 1,000 / 20,000 / 100,000，空间数 1 / 3 / 10；账号空间容量取其有效空间套餐的最大值，降级保留已有空间但禁止超额新建。
- 用户列表分页/搜索/状态/角色筛选；启停、升降管理员、强制改密和撤销会话。治理事务串行保护至少一名启用站长；角色/状态/改密要求均撤会话，新会话仍受账号版本校验。
- 空间列表分页/搜索/状态筛选、归属和当月用量；套餐编辑、人工开通、空间停用/恢复均与平台审计同事务提交。
- 独立 `platform_redemptions` / `platform_redemption_uses`：批量 1–100 个、每码 1–1,000 次、有效期、作废、使用记录；原文仅生成时返回，数据库存摘要和尾号，审计不写原文。
- 兑换需选本人且未停用的空间，限制尝试频率；每码每空间最多兑换一次。全局次数用条件更新扣减，开通/使用记录/审计与扣减共同提交或回滚；失败统一响应。
- `plan_assignments` 记录 `manual` / `redeem`、操作账号、兑换记录与到期时间。同套餐未过期从当前到期顺延，过期或更换套餐从现在起算；自然月末向目标月末截断。停用须另行恢复，开通套餐不能绕过停用。
- 保留已有平台会话与 CSRF 契约；敏感操作要求五分钟内重新认证，重新认证轮换当前会话和 CSRF。强制改密账号仅可查看会话、改密和退出。公开 Origin 按配置严格校验，缺 Origin 使用同源 Referer；调试日志改用现有日志器且不记录凭据。

### 已验收：独立后台与用户控制台

- `/platform` 默认展示本人空间、套餐能力矩阵、空间容量和兑换入口；`/platform/admin/{users,workspaces,plans,redemptions,audits}` 分页提供站长管理。用户页支持邮箱搜索、状态和角色筛选，空间与兑换码支持搜索及状态筛选。
- 普通用户直接打开后台 URL 显示权限不足，不加载后台数据；强制改密账号只显示改密弹窗。已修复后台直链登录后无法恢复页面的问题。
- 用户治理、空间启停、人工套餐及套餐修改复用 `ConfirmDialog`；生成兑换码复用 `Dialog` / `CopyButton`，完整码仅当次显示，使用记录单独分页。敏感操作过期时可通过后台的重新认证入口轮换会话后重试。
- 表格复用 `DataTablePage` 的搜索、防抖、分页、移动端卡片、空态和加载态；平台组件负责自己的数据和权限，不接入空间内的管理员账号或额度兑换 API。

### 已验收：翻译与并发续期修复

- 77 个新增翻译键通过项目脚本写入 en、zh、zh-TW、fr、ja、ru、vi，共 539 项；`i18n:sync` 和全前端键扫描通过。数值表单错误也使用可翻译的区间提示。
- 实测复现 MySQL 默认 REPEATABLE READ 下，兑换事务读取旧空间快照导致并发人工续期被覆盖；空间锁定后改用当前锁定读，账号及治理读取共用跨数据库锁定规则。确定性交错测试先失败再通过，SQLite 保持已有写事务串行化。
- 初始化管理员凭据只在没有启用站长时使用。已任命其他站长后，原初始化账号的降权不会导致重启失败，也不会被环境变量重新提权；回归按实际启动顺序重新打开数据库验证角色保持。

### 已验收：第二阶段真实数据库迁移

2026-09-12，SQLite 3.50.4（Go 驱动）、MySQL 8.4.11、PostgreSQL 17.11（Debian 17.11-0+deb13u1）均完成以下矩阵，每种路径分别启动第二阶段两次，共 18 次启动。MySQL/PostgreSQL 同时配置独立关系型日志库。

| 路径 | SQLite | MySQL | PostgreSQL |
| --- | --- | --- | --- |
| 第二阶段新库 + 再次启动 | 通过 | 通过 | 通过 |
| 第一阶段 `a0218c0a3` 建库 → 升级 + 再次启动 | 通过 | 通过 | 通过 |
| 最新发布版 `v1.0.0-rc.37` 建库 → 升级 + 再次启动 | 通过 | 通过 | 通过 |

- 旧平台会话可继续使用；新增用户状态和会话版本回填正确。旧人工开通记录补上 `manual` 来源和操作人。
- 旧 Pro ID 2、明确修改的价格/限额、页脚开关、空间容量和未知扩展能力均保留；Standard 新增，缺失能力补齐。
- users/tokens/channels/abilities/options 及旧平台用户、空间、人工开通记录摘要一致；旧 root 可登录。独立日志库消费额度 21 和审计记录保留。第二次启动的表定义、列、唯一索引和其他索引保持一致。
- 新库实际注册普通用户并确认后台 API 403；兑换 Standard 后空间容量增至 3、品牌能力开启；重复兑换失败，重启后用码次数、使用记录和审计仍精确为 1。

命令：`python3 /tmp/saas-phase2-matrix.py`，输出 `/tmp/saas-phase2-matrix-results.log`；临时库前缀 `saas_phase2_accept_0912b`。第一阶段二进制从当前 HEAD 的独立临时检出构建；发布版二进制来自已核对标签的 `/tmp/saas-release`。脚本从权限受限的 `/tmp` 文件读取随机凭据，仓库不保存凭据。

### 已验收：第二阶段构建与回归

- `go test ./...`、`go test -race . -run TestSaaSContracts -count=1`、最终 `go vet ./...` 通过；SQLite、MySQL、PostgreSQL 的完整 SaaS 契约均包含第二阶段用例，初始化管理员重启与 MySQL 并发续期回归通过。
- 前端 `bun run test --maxWorkers=2 --testTimeout=15000`：**138 个文件 / 1,487 项测试通过**；平台/品牌专项 5 文件 / 17 项通过，`bun run typecheck`、30 个修改 TS/TSX 文件的 lint 与格式检查通过。
- `bun run build:check` 通过；使用最新 `web/dist` 执行 `go build -o /tmp/new-api-saas-phase2 .` 成功。生产分包总代码约 59.0 MB（gzip 17.1 MB，含按需加载资源）。
- 最终三数据库契约命令使用本机临时库，未包含生产密码：

```bash
go test . -run TestSaaSContracts -count=1
SAAS_TEST_DSN='root@tcp(127.0.0.1:33306)/saas_phase2_final_0912c?charset=utf8mb4&parseTime=true' \
SAAS_TEST_LOG_DSN='root@tcp(127.0.0.1:33306)/saas_phase2_final_0912c_log?charset=utf8mb4&parseTime=true' \
go test . -run TestSaaSContracts -count=1
SAAS_TEST_DSN='postgres://box@127.0.0.1:35432/saas_phase2_final_0912c?sslmode=disable' \
SAAS_TEST_LOG_DSN='postgres://box@127.0.0.1:35432/saas_phase2_final_0912c_log?sslmode=disable' \
go test . -run TestSaaSContracts -count=1
```

验收输出在 `/tmp/saas-phase2-go-full-final.log`、`/tmp/saas-phase2-race-final.log`、`/tmp/saas-phase2-vet-final.log`、`/tmp/saas-phase2-{mysql,postgres}-final2.log`、`/tmp/saas-phase2-web-full.log`、`/tmp/saas-phase2-web-build.log`。

### 已验收：第二阶段公开演示

2026-09-12，演示实例使用最终前端资源与 Go 构建启动，`PLATFORM_ORIGIN=https://newapi-sass.moonrend.com`。升级前保留一致性 SQLite 备份和权限 0600 的环境文件。Chromium 153.0.8010.12 实际浏览器验证：

1. 站长通过 `/platform/admin/users` 深链接登录后直接进入后台；普通用户注册登录后没有后台导航。直接后台路由显示权限不足，用户、空间、套餐、兑换码、审计五类后台接口均返回 403。
2. Lite 创建 Alpha 后容量为 1/1，创建表单禁用；root 一次性激活及加密登录成功。空间显示强制托管页脚，Lite 的品牌写入在服务端返回 403。
3. 后台批量生成三个 Standard 码，完整码只在生成弹窗显示。普通用户兑换及同套餐续期成功，到期时间顺延、容量升至 3，品牌开放而页脚锁定；可继续创建 Beta。重复兑换返回统一失败。
4. 站长人工开通 Pro 后容量变为 10，品牌与页脚能力立即生效。停用 Alpha 后接口返回 403，Beta 仍为 200；显式恢复后 Alpha 可用。
5. 站长作废未使用码，普通用户看到统一失败文案；使用记录对应正确空间，平台审计可查开通、兑换和作废操作。
6. 站长对测试账号要求改密后旧会话失效；再次登录仅能改密，空间 API 返回 `password_change_required` / 403。改密并重新登录后恢复控制台。套餐编辑弹窗加载正常，390px 中文移动端没有横向溢出。

截图：[普通用户控制台](../.github/assets/saas-phase2-workspaces.png)、[站长空间管理](../.github/assets/saas-phase2-administration.png)。截图只包含独立创建的演示数据；密码、会话、root 激活链接和完整兑换码不入仓库。浏览器脚本位于 `/tmp/saas-browser/phase2-browser.mjs`，输出为 `/tmp/saas-phase2-browser-{init,ops,ops-finish,history,final}.log`；脚本调试时修正了 HTTP 状态和表格选择器断言，产品流程最终通过。

### 第二阶段安全与后续范围

- 延续上文 OWASP Authentication、Session Management、Password Storage、CSRF Prevention 与 ASVS 5.0.0 依据。新增回归覆盖最后站长保护、普通用户越权、错误 Origin/Referer、敏感操作重新认证与会话轮换、账号版本失效、强制改密、兑换限流/过期/重放/全局次数、跨用户兑换拒绝及审计失败事务回滚。
- 代码、文档与截图检查未发现实际凭据；既有项目版权头保持一致。平台审计仅记录账号/空间/套餐 ID、来源、状态变化及时间，不写密码或可用码。
- 规格允许后续实现的平台在线支付、平台 MFA、会话列表尚未加入；当前提供密码重新认证与按账号撤销全部平台会话。三档套餐均可人工或兑换码开通。
