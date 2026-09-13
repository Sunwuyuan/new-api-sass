# New API SaaS — 第三阶段：平台 UI 专业化（登录优先）

硬约束不变：单进程 + `tenant_id`；禁止每租户 worker；少改 relay/渠道核心；密钥不进仓库；双层账号（平台账号 ≠ 空间内 New API 用户）。

## 问题
当前 `/platform` 把登录/注册塞在同一页简陋 Card 里，没有独立路由，也没有 New API 已支持的 OAuth / Passkey / 微信等。整体观感像玩具，不像可运营 SaaS。

## 目标（本阶段优先）
1. **平台认证做成独立页面**，路由与体验对齐 New API `/(auth)/*`：
   - `/platform/sign-in`
   - `/platform/sign-up`
   - 需要时：`/platform/forgot-password`、`/platform/reset`、`/platform/otp`、`/platform/oauth`、`/platform/oauth/$provider`（回调）
   - 未登录访问 `/platform` 或受保护页 → 跳到 `/platform/sign-in?redirect=...`
   - 已登录访问登录/注册 → 跳回工作台
2. **登录/注册 UI 直接复用 New API 风格与内容结构**，不要另起一套玩具表单：
   - 复用 `features/auth/auth-layout.tsx`（或抽共享 layout，平台 logo/名称可用平台品牌配置）
   - 复用 `UserAuthForm` / `SignUpForm` 的布局与交互模式：密码框、OAuth 区、Passkey、微信码、条款页脚、注册入口文案
   - 文案/i18n 走同一套键；视觉密度、间距、按钮层级与空间内登录页一致
   - **禁止**再在 dashboard 中间塞登录 Card
3. **OAuth 与其它登录能力要对齐 New API 能力面**（平台层）：
   - GitHub / Discord / OIDC / LinuxDo / Telegram / WeChat / Passkey（以及 New API 已有的自定义 OAuth 若可复用）
   - 配置作用域在**平台级**（不是某个租户的 system setting）；可用环境变量或 `platform_settings` 表，站长后台可后续编辑，本阶段至少：后端可读配置 + 前端 status 接口暴露开关 + UI 按开关显示按钮
   - 回调落到平台会话（`platform` cookie/session），**不要**写成空间内 `model.User` 登录
   - 参考现有 `controller`/`oauth` 与 `features/auth` 实现，抽或复制最小必要逻辑到 `platform/`，避免大改 upstream OAuth 核心；能共享的纯函数放独立小包
4. **登录后壳子专业化（本阶段顺手做，勿喧宾夺主）**：
   - 工作台 / 站长后台：左侧或顶栏清晰信息架构（参考 New API 后台侧栏密度），去掉「一排 pill + 大片空白」玩具感
   - 套餐对比可折叠/次要入口，不要登录后第一屏就三大价卡压住工作区
   - 保留功能：兑换码、人工开通、审计、三档能力差；只改 IA 与视觉层级

## 非目标
- 不做在线支付
- 不合并平台账号与空间账号
- 不引入每租户 worker
- 不要求一次做完所有品牌主题编辑器

## 工程
- 分支：`feat/saas-multitenant` 继续，或 `feat/saas-platform-ui`
- 更新 `docs/SAAS_STATUS.md` 第三阶段段
- 前端测：登录路由跳转、未登录保护、OAuth 按钮按 status 显隐（可 mock）
- 后端：平台 OAuth/Passkey 相关 API + 测试；`go test` 相关包
- 本机构建前端；演示 `https://newapi-sass.moonrend.com` 用新构建重启后回归
- commit + push 更新 PR #1（或新 PR）

## 验收
1. `/platform/sign-in`、`/platform/sign-up` 为独立全屏页，观感接近 New API 空间登录页
2. 开启的平台 OAuth/Passkey/微信等在登录页可见并可走通（至少一种 OAuth e2e 或契约测 + UI 测；其余有 status 驱动显隐）
3. 未登录不能看工作台/后台；登录后不再看到嵌入式登录表单
4. 架构仍单进程 + tenant_id；密钥不进仓


## 补充（用户 2026-09-12）：完整页面 / 分实例管理 / 用量分析

1. **不同页面都要完整**\
   平台侧每个功能做成独立完整页面（路由+布局+空态/加载/错误/操作），不要把登录、创建空间、套餐、兑换、管理挤在同一滚动长页里凑合。登录/注册已要求独立；工作台、空间详情、站长各模块同理。

2. **不同实例单独显示管理页面**\
   每个空间（tenant/instance）有自己的管理入口与详情页，例如 `/platform/workspaces/{id}` 或 `/platform/workspaces/{slug}`：展示该实例的套餐、用量、状态、到期、入口链接、兑换/升级操作；站长也可进入对应实例管理视图。不要只用一张大表/几张卡片带两个按钮完事。

3. **用量分析显示**\
   平台层提供用量分析视图（至少）：\
   - 当前用户：各空间月请求用量、限额、剩余、到期；可图表或清晰进度条+表格\
   - 站长：全站/按空间聚合用量、套餐分布、近期待办（可先做汇总卡+可筛选表，图表有余力再加）\
   - 数据来源沿用已有 plan usage / 日志计数；勿编造指标。路由建议：`/platform/usage`（用户）、`/platform/admin/usage`（站长）

优先级：认证独立页+复用 New API auth → 分实例管理页 → 用量分析 → 壳子专业化。
