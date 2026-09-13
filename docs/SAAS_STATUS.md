# SaaS 实施状态

目标和产品决策以 [SAAS_SPEC.md](SAAS_SPEC.md) 及 [SAAS_PHASE2_SPEC.md](SAAS_PHASE2_SPEC.md) 为准。**禁止偏离。**

## 架构硬约束（不可改）
- 同一套部署、同一进程（可水平多副本），共享同一数据库
- `/t/{slug}` 中间件注入 `tenant_id`；数据层强制 scope；缓存/会话隔离
- **禁止**每租户独立工作进程 / supervisor / Bound worker
- 平台与空间双层账号；设置按租户；包月限额；人工开通套餐；少改 relay/渠道

## 当前状态

第一至第三阶段实现与验收已完成，交付分支为 `feat/saas-multitenant`，集中更新 [PR #1](https://github.com/Sunwuyuan/new-api-sass/pull/1)。三数据库迁移、Go/前端测试、生产构建及公开演示回归均通过；平台独立认证、分实例管理、用量分析与侧栏工作台已上线。演示入口为 [平台控制台](https://newapi-sass.moonrend.com/platform)。没有单租户模式开关。

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


## 第三阶段（实现与验收完成）

见 [SAAS_PHASE3_UI_SPEC.md](SAAS_PHASE3_UI_SPEC.md)。

**用户补充（必做）**：① 各功能独立完整页面；② 每个空间实例独立管理/详情页；③ 用量分析 `/platform/usage` 与 `/platform/admin/usage`。优先级：认证独立页+复用 New API auth/OAuth → 分实例管理 → 用量分析 → 壳子专业化。

### 已验收：平台认证基础

- `platform/` 独立读取平台品牌、注册开关、GitHub / Discord / LinuxDO / OIDC / Telegram、自定义 OAuth、微信及 Passkey 环境配置；`/platform/api/status` 只返回公开开关与品牌信息。
- 新增平台身份、一次性认证流程和 Passkey 表；回调仅签发平台会话。OAuth 使用 S256 PKCE、浏览器绑定 state、独立回调地址；OIDC 验证签名、issuer、audience、nonce。禁止按外部邮箱自动合并账号，关联已有账号需要有效会话与重新认证。
- Passkey 使用平台 Origin、必需用户验证、独立用户句柄和一次性 challenge；增加登记、删除和重新认证接口。微信复用既有验证码服务契约，并在平台增加浏览器绑定和短期重放保护。
- 平台 OAuth/OIDC、Passkey、微信、实例用量契约及三数据库迁移通过，实际浏览器完成平台密码和 Passkey 登录；验证范围见下文。
- 已抽出 New API 共享认证框架、OAuth 按钮和微信验证码弹窗；平台与空间分别传入自己的配置和操作，不共享登录状态。

### 已验收：独立页面与实例用量

- `/platform/sign-in`、`/platform/sign-up` 使用抽出的 New API `AuthLayoutFrame`、OAuth 按钮、密码框、条款与微信弹窗；路由加载前验证平台会话，保留安全的站内返回地址。
- 登录后使用共享 Sidebar 壳子，工作台优先展示本人空间；创建、兑换、套餐、账号安全均为独立页面。原五个站长模块继续提供分页与治理操作。
- 新增 `/platform/workspaces/:id` 和站长实例详情，展示套餐、到期、状态、月用量、最近六个月实际记录及最近二十次套餐变更；复用兑换弹窗、人工套餐和确认操作。
- `/platform/usage`、`/platform/admin/usage` 提供按空间进度表、真实月计数、套餐分布、到期/停用/额度耗尽汇总，后端先限定空间所有权。沿用 `tenant_usage`，未改 relay 或引入新 worker。
- 新增集中认证契约测试，实际执行 OAuth PKCE 令牌交换、OIDC 签名及 `auth_time` 校验、Passkey CBOR 注册与 P-256 签名登录；覆盖浏览器绑定、过期/重放、跨账号关联与删除、强制改密、停用、实例越权和用量聚合。
- 全量 Go/前端测试、真实数据库矩阵、生产构建及公开演示验收通过。最终可访问性修正消除嵌套主内容区，空间入口保留链接语义和停用状态。

### 已验收：第三阶段数据库与迁移修复

- 初轮启动检查通过后，追加旧库实际插入外部账号的探针，复现 GORM 不会自动放宽旧 `platform_users.email NOT NULL` 的问题。已在 `New` 中增加显式邮箱迁移；SQLite 使用驱动事务内重建表并恢复原索引/触发器，MySQL/PostgreSQL 使用各自 GORM 列迁移。
- 新增旧模型回归，验证两次启动后既有账号内容、邮箱唯一性、自定义索引和 SQLite 触发器均保留，并可创建多个没有平台邮箱的第三方账号。
- SQLite 3.50.4、MySQL 8.4.11、PostgreSQL 17.11 的平台认证/实例用量/迁移契约均通过；MySQL/PostgreSQL 的完整 `TestSaaSContracts` 与独立关系型日志库也通过。
- 修复后完整重跑新库、第二阶段 `caf4f14bf` 旧库、最新发布版 `v1.0.0-rc.37` 旧库矩阵，各启动两次，共 18 次。每次启动均实际验证多个 NULL 邮箱账号、原邮箱唯一性、旧业务摘要、平台会话、方案能力、兑换历史、主/日志库表与索引幂等性。最新发布标签已通过 GitHub API 再次核对。
- 命令：`python3 /tmp/saas-phase3-matrix.py`；临时库前缀 `saas_phase3_accept_0912b`；结果 `/tmp/saas-phase3-matrix-results-2.log`。专项测试使用 `PLATFORM_TEST_DSN` 指向 `saas_phase3_auth_0912a`；完整契约使用 `SAAS_TEST_DSN` / `SAAS_TEST_LOG_DSN` 指向 `saas_phase3_contract_0912a` / `_log`，连接端口与前阶段相同。

### 已验收：第三阶段界面交互与翻译

- 54 个新增文案通过规定脚本写入七种语言，共 378 项；`i18n:sync` 与全前端键扫描通过。
- 平台前端专项 4 个文件 / 29 项通过：独立登录/注册、深链接恢复、匿名及普通用户后台保护、status 驱动的全部登录方式显隐、OAuth query/fragment 编码保真与单次回调、安全返回地址、微信键盘提交、Passkey 浏览器请求、实例详情/用量和既有治理操作。
- OAuth 按钮装饰图标不再重复读出提供方名称；实例详情与工作台补齐套餐加载错误状态。前端全量 138 个文件 / 1,500 项通过；最后回调 fragment 与可访问性修正后，平台专项 4 个文件 / 29 项、类型检查、42 个 TS/TSX 文件 lint/格式检查和生产构建再次通过。

### 已验收：第三阶段公开演示

演示地址：[平台登录](https://newapi-sass.moonrend.com/platform/sign-in)。使用 Chromium 153.0.8010.12，真实运行服务及数据库，不替换平台 API 响应。

1. 独立登录/注册页、注册后的登录提示、受保护深链接恢复、未配置提供方隐藏、普通用户后台路由拒绝；用户/空间/套餐/兑换码/审计/用量六类管理接口均返回 403。
2. 创建 Alpha、Lite 空间容量限制、独立实例详情和工作台优先展示空间；实际激活空间 root 并通过原 New API 登录页登录，平台和空间会话保持独立。
3. 使用 Chromium 虚拟认证器完成真实 WebAuthn 注册、刷新持久化、平台退出、Passkey 登录、重新验证和删除；平台退出后原空间登录仍有效。
4. 独立兑换页为 Alpha 开通 Standard；站长从实例管理页人工开通 Pro，容量即时更新并可创建 Beta，实例详情保留两次套餐记录。
5. 停用 Alpha 后其状态接口返回 403，Beta 保持 200；恢复成功，非本人空间详情返回 404。本人/站长用量、五个既有管理模块及用户搜索均通过。
6. 中文 390px 手机页面覆盖登录、注册、工作台、创建、兑换、套餐、安全、实例详情、用量及所有后台模块；没有页面横向溢出，宽表在自身容器内滚动。手机侧栏可打开、导航并关闭；1440px 桌面验收通过，没有浏览器运行时错误。
7. 最终构建重启后，验证 OAuth query 先 303 移入 fragment；包含 Cloudflare 预取在内的 18 个同源资源/API 请求 Referer 均不再包含授权参数。回调页面清除 URL 参数，CSP 保留，实例页只有一个主内容地标且入口为正确链接。

用量展示来自 `tenant_usage`；此演示账号尚未发起网关调用，页面如实显示 0 和历史空态，非零汇总由精确数据库契约覆盖。脚本为 `/tmp/saas-browser/phase3-browser.mjs`（`auth` / `management`）、`phase3-visual.mjs`、`phase3-callback-check.mjs`；结果为 `/tmp/saas-phase3-browser-{auth,management}.log`、`/tmp/saas-phase3-visual-final.log`、`/tmp/saas-phase3-callback-browser.log`。最终演示二进制为 `/tmp/new-api-saas-phase3-accepted`，一致性备份留在权限 0600 的仓库外文件中。

可审查截图：[独立登录](../.github/assets/saas-phase3-sign-in.png)、[工作台](../.github/assets/saas-phase3-workspaces.png)、[实例详情](../.github/assets/saas-phase3-workspace.png)、[站长用量](../.github/assets/saas-phase3-administration.png)、[手机登录](../.github/assets/saas-phase3-sign-in-mobile.png)、[手机实例管理](../.github/assets/saas-phase3-workspace-mobile.png)。截图不包含密码、可用会话、激活链接或完整兑换码。

### 第三阶段验证命令

```bash
go test ./...
go vet ./...
go test -race ./platform . -run 'TestPlatform|TestSaaSContracts' -count=1
# 最后补齐 OIDC iat、回调 fragment 与页面策略后
go test ./platform . ./router -count=1
go vet ./platform ./router .

# SQLite 3.50.4（Go 驱动）
go test ./platform -count=1

# MySQL 8.4.11 / 33306
PLATFORM_TEST_DSN='root@tcp(127.0.0.1:33306)/saas_phase3_auth_0912a?charset=utf8mb4&parseTime=true' \
go test ./platform -count=1
SAAS_TEST_DSN='root@tcp(127.0.0.1:33306)/saas_phase3_contract_0912a?charset=utf8mb4&parseTime=true' \
SAAS_TEST_LOG_DSN='root@tcp(127.0.0.1:33306)/saas_phase3_contract_0912a_log?charset=utf8mb4&parseTime=true' \
go test . -run TestSaaSContracts -count=1

# PostgreSQL 17.11 / 35432
PLATFORM_TEST_DSN='postgres://box@127.0.0.1:35432/saas_phase3_auth_0912a?sslmode=disable' \
go test ./platform -count=1
SAAS_TEST_DSN='postgres://box@127.0.0.1:35432/saas_phase3_contract_0912a?sslmode=disable' \
SAAS_TEST_LOG_DSN='postgres://box@127.0.0.1:35432/saas_phase3_contract_0912a_log?sslmode=disable' \
go test . -run TestSaaSContracts -count=1

cd web
bun run test --maxWorkers=2 --testTimeout=15000
bun run test src/features/platform/__tests__ --maxWorkers=2 --testTimeout=15000
bun run typecheck
bun run build:check
cd ..
go build -o /tmp/new-api-saas-phase3-accepted .
```

上述数据库是可重建的本机临时测试库。全量日志为 `/tmp/saas-phase3-go-full.log`、`/tmp/saas-phase3-go-vet.log`、`/tmp/saas-phase3-auth-race.log`、`/tmp/saas-phase3-web-full.log`；最终生产构建 `/tmp/saas-phase3-web-build-final.log`，产物合计 59.2 MB（gzip 17.2 MB，含按需加载资源）。

### 第三阶段安全依据与运行边界

- 实施前阅读 OWASP [Authentication](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)、[Session Management](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)、[OAuth2](https://cheatsheetseries.owasp.org/cheatsheets/OAuth2_Cheat_Sheet.html)、[MFA](https://cheatsheetseries.owasp.org/cheatsheets/Multifactor_Authentication_Cheat_Sheet.html)、[CSRF](https://cheatsheetseries.owasp.org/cheatsheets/Cross-Site_Request_Forgery_Prevention_Cheat_Sheet.html)、[Password Storage](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) 指南及 **ASVS 5.0.0**。重点验证 V6.8.1/V6.8.2/V6.8.4（外部身份命名空间、签名与认证新鲜度）、V10.2.1/V10.2.2/V10.2.3（OAuth CSRF、混淆攻击防护和最小 scope）和 V7.2.4（重新认证后的会话轮换）；沿用前阶段密码、会话与权限控制，不代表整个项目已通过全面 ASVS 认证。
- OAuth/OIDC 使用固定平台回调、S256 PKCE、五分钟浏览器绑定单次 state，身份按提供方/客户端/issuer 命名空间隔离；ID Token 校验签名、issuer、audience、azp、nonce、expiry、必需 `iat` 及未来签发时间。失败、过期、重放、错浏览器、错身份、停用及改密绕过均有回归。
- Passkey 使用平台 RP Origin、强制用户验证和平台专属 user handle；凭据修改要求近期认证，删除保护最后登录方式并撤销旧会话。已有空间 Passkey 配置和会话不参与平台认证。
- 公开代理覆盖 `Referrer-Policy` 与 `X-Frame-Options`，且 Cloudflare `Speculation-Rules` 在 HTML meta 生效前发起请求。真实 Referer 探针复现泄漏后，回调改为先 303 将 query 移入 fragment，再提供 HTML，客户端单次提交并清除参数；重新验证边缘预取和全部资源/API 请求不携带授权参数。保留 `no-referrer` meta、`Content-Security-Policy: frame-ancestors 'none'` 和应用访问日志 query 脱敏。
- 演示已启用 Passkey；未提供真实 OAuth/微信凭据，相关按钮按 status 隐藏。OAuth/OIDC/微信完成后端契约及前端显隐/回调测试，未声称完成真实外部提供方授权。微信桥接服务须保证验证码五分钟内过期且单次消费，平台另外提供数据库重放保护。
- OIDC 重新认证要求提供方返回新鲜且签名可信的 `auth_time`；普通 OAuth userinfo 不作为强制重新认证方式。账号安全页按已关联能力提供密码、Passkey、微信或支持验证的 OIDC 入口。邮箱找回、OTP/MFA、在线支付不在本阶段范围。
- 认证配置仅来自平台环境变量，状态 API 不返回密钥；具体配置与回调见 [中文 README](../README.zh_CN.md)。复用 New API AuthLayoutFrame、OAuthProviderButtons、密码输入、条款和微信弹窗；原表单绑定空间 store/API，平台组合共享组件并调用独立平台接口，以保留双层账号。
