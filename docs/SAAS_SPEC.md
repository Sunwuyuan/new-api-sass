# New API SaaS — 多租户托管规格（必须完整实现）

仓库：`Sunwuyuan/new-api-sass`（基于 QuantumNous/new-api，在现有代码上改，不另起项目）

## 产品目标
把 New API 做成多租户 SaaS：每人可创建独立「空间」，空间内体验尽量等同独立 New API；平台收托管费（包月套餐），不碰租户上游成本。

## 已定决策（不要再改）
1. **上游渠道/Key**：租户自己在空间内配置；**不要改** New API 原有上游管理逻辑，只加租户隔离。
2. **入口**：`/t/{slug}/...` 路径前缀；代码结构预留日后子域名，但第一版只做路径。
3. **身份：双层账号**
   - 平台账号：注册/登录、创建空间、看套餐与用量、平台管理人工开通套餐。
   - 空间内：仍用 New API 自有登录与权限。创建空间时自动在该租户下创建 root，把初始密码或重置链接交给创建者。
4. **系统设置**：按租户隔离（站点名、注册开关、SMTP、页脚、品牌等），尽量独立站体验。
5. **计费**：固定套餐包月；按请求量等限额；超限需升级或停用。
6. **支付**：第一版**不做**在线支付；套餐由平台管理员人工开通。
7. **不要** SAAS_MODE / 单租户开关；产品就是多租户 SaaS。
8. **可维护性**：少改 `relay/` 与渠道核心；SaaS 逻辑放独立包（如 `tenant/`、`platform/`、`plan/`）；用中间件 + DB scope 强制 `tenant_id`，便于后续合并上游。
9. **套餐能力矩阵**：不同套餐可改的设置不同。例如 Lite **不能**移除平台页脚；前后端都必须强制校验（以后端为准）。
10. **隔离硬要求**：不同租户数据绝对不能互相影响（DB / cache / session / upload / 异步任务 key 全部带租户维度）。

## 第一版必须交付（可用）
### A. 数据模型
- `platform_users`（平台账号）
- `tenants`（slug、name、owner_platform_user_id、plan_id、status、created_at…）
- `plans`（name、price 展示字段、limits JSON、capabilities JSON）
- `tenant_usage` / 计量计数（按月请求量等）
- `plan_assignments` 或等价人工开通记录
- 所有租户业务表加 `tenant_id`（users、tokens、channels、logs、options/settings、redemptions 等凡属空间的数据）
- 迁移脚本可在 SQLite/MySQL/PostgreSQL 上跑（与上游一致）

### B. 租户解析与隔离
- 路由：`/t/:slug/*` 进入租户上下文；平台路由在 `/platform/*`（或 `/` 平台门户 + `/t/:slug` 空间）
- 中间件解析 slug → tenant；无租户上下文访问租户 API 必须失败
- GORM/查询层默认强制 `tenant_id`；缺失上下文应报错而非全表扫描
- Redis/内存缓存 key 前缀含 tenant
- 会话 cookie 按租户隔离（path 或 name 区分），避免串会话

### C. 平台控制台（最小可用 UI）
- 平台注册/登录
- 创建空间（选 slug、套餐申请/默认免费档）
- 空间列表、用量、套餐状态
- 平台管理员：用户/租户列表、人工开通/变更套餐、停用租户
- 进入空间：跳到 `/t/{slug}/`（空间内用原 New API UI）

### D. 空间内
- 现有 New API UI 尽量复用，挂在 `/t/{slug}/` 下（前端 router basename / API base 适配）
- 渠道、令牌、用户、日志等功能保留且租户隔离
- Option 读写包装：按租户存；保存前校验套餐 capabilities
- 平台页脚：无 `remove_platform_footer` 能力时强制显示且不可清空

### E. 套餐与计量
默认内置至少两档（可在 DB seed）：
- **Lite**：月请求上限较低；`remove_platform_footer=false`；用户/令牌/渠道上限较低
- **Pro**：更高限额；允许自定义/移除平台页脚等

计量：每个进入 relay/网关的成功或计费请求计入该租户当月用量；达上限后拒绝新请求（明确错误码/文案），管理员可改套餐后恢复。

### F. 工程与文档
- `README.zh_CN.md` 或 `docs/SAAS_README.md`：如何配置 DB、启动、创建第一个平台管理员、创建空间、人工开通套餐
- `docker-compose` 能一键起（沿用/扩展现有）
- 基本测试：跨租户隔离（同资源 id 不同 tenant → 404）；Lite 不能去页脚；超限拒绝
- 提交到 GitHub：推到分支 `feat/saas-multitenant`，并创建 PR 到 `main`；提交信息清晰
- **禁止**把 API Key、密码写进仓库

## 明确不要做
- 不要重写上游渠道管理
- 不要做在线支付
- 不要做一库一租户 / schema-per-tenant
- 不要保留单租户兼容模式开关

## 验收标准（明早可用）
1. `docker compose up`（或文档中的等价命令）能启动
2. 能注册平台账号 → 创建两个空间 → 各自登录空间 root → 数据互不可见
3. Lite 租户无法去掉平台页脚；Pro（人工开通后）可以按能力矩阵允许的项修改
4. 模拟打满请求限额后网关拒绝；开通更高套餐后恢复
5. 代码结构便于继续 merge QuantumNous/new-api

实现时先读现有 `model/`、`middleware/`、`router/`、`web/` 结构再改；优先可运行，再补测试与文档。
