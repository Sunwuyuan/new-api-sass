<div align="center">

![new-api](/web/public/logo.png)

# New API

🍥 **新一代大模型网关与AI资产管理系统**

<p align="center">
  简体中文 |
  <a href="./README.zh_TW.md">繁體中文</a> |
  <a href="./README.md">English</a> |
  <a href="./README.fr.md">Français</a> |
  <a href="./README.ja.md">日本語</a>
</p>

<p align="center">
  <a href="https://raw.githubusercontent.com/Calcium-Ion/new-api/main/LICENSE">
    <img src="https://img.shields.io/github/license/Calcium-Ion/new-api?color=brightgreen" alt="license">
  </a><!--
  --><a href="https://github.com/Calcium-Ion/new-api/releases/latest">
    <img src="https://img.shields.io/github/v/release/Calcium-Ion/new-api?color=brightgreen&include_prereleases" alt="release">
  </a><!--
  --><a href="https://hub.docker.com/r/CalciumIon/new-api">
    <img src="https://img.shields.io/badge/docker-dockerHub-blue" alt="docker">
  </a>
  <a href="https://atomgit.com/QuantumNous/new-api" target="_blank">
    <img alt="AtomGit G-Star" src="https://atomgit.com/QuantumNous/new-api/star/badge.svg"/>
  </a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/20180" target="_blank">
    <img src="https://trendshift.io/api/badge/repositories/20180" alt="QuantumNous%2Fnew-api | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/>
  </a>
  <br>
  <a href="https://hellogithub.com/repository/QuantumNous/new-api" target="_blank">
    <img src="https://api.hellogithub.com/v1/widgets/recommend.svg?rid=539ac4217e69431684ad4a0bab768811&claim_uid=tbFPfKIDHpc4TzR" alt="Featured｜HelloGitHub" style="width: 250px; height: 54px;" width="250" height="54" />
  </a><!--
  -->
  <a href="https://atomgit.com/QuantumNous/new-api" target="_blank">
    <img alt="AtomGit G-Star" src="https://atomgit.com/QuantumNous/new-api/star/new_badge.svg" width="250" height="55" />
  </a>
</p>

<p align="center">
  <a href="#-快速开始">快速开始</a> •
  <a href="#-主要特性">主要特性</a> •
  <a href="#-部署">部署</a> •
  <a href="#-文档">文档</a> •
  <a href="#-帮助支持">帮助</a>
</p>

</div>

## 📝 项目说明

> [!IMPORTANT]
> - 本项目仅面向合法授权的 AI API 网关、组织内部鉴权、多模型管理、用量统计、成本核算和私有化部署场景。
> - 使用者必须合法取得上游 API Key、账号、模型服务或接口权限，并遵守上游服务条款及适用法律法规。
> - 使用者应确保其使用方式符合上游服务条款及适用法律法规。
> - 面向公众提供生成式人工智能服务时，使用者应遵守[《生成式人工智能服务管理暂行办法》](http://www.cac.gov.cn/2023-07/13/c_1690898327029107.htm)等监管要求，自行完成所在司法辖区要求的备案、许可、内容安全、实名、日志留存、税务和上游授权等合规义务。

---

## 🤝 我们信任的合作伙伴

<p align="center">
  <em>排名不分先后</em>
</p>

<p align="center">
  <a href="https://www.cherry-ai.com/" target="_blank">
    <img src="./docs/images/cherry-studio.png" alt="Cherry Studio" height="80" />
  </a><!--
  --><a href="https://github.com/iOfficeAI/AionUi/" target="_blank">
    <img src="./docs/images/aionui.png" alt="Aion UI" height="80" />
  </a><!--
  --><a href="https://bda.pku.edu.cn/" target="_blank">
    <img src="./docs/images/pku.png" alt="北京大学" height="80" />
  </a><!--
  --><a href="https://www.compshare.cn/?ytag=GPU_yy_gh_newapi" target="_blank">
    <img src="./docs/images/ucloud.png" alt="UCloud 优刻得" height="80" />
  </a><!--
  --><a href="https://www.aliyun.com/" target="_blank">
    <img src="./docs/images/aliyun.png" alt="阿里云" height="80" />
  </a><!--
  --><a href="https://io.net/" target="_blank">
    <img src="./docs/images/io-net.png" alt="IO.NET" height="80" />
  </a>
</p>

---

## 🙏 特别鸣谢

<p align="center">
  <a href="https://www.jetbrains.com/?from=new-api" target="_blank">
    <img src="https://resources.jetbrains.com/storage/products/company/brand/logos/jb_beam.png" alt="JetBrains Logo" width="120" />
  </a>
</p>

<p align="center">
  <strong>感谢 <a href="https://www.jetbrains.com/?from=new-api">JetBrains</a> 为本项目提供免费的开源开发许可证</strong>
</p>

---

## 🚀 快速开始

### 本分支：New API SaaS

本分支基于 QuantumNous/new-api，入口是平台控制台 `/platform` 和空间 `/t/{slug}/`。一个 Go 进程服务所有空间，共享数据库；平台账号与空间内的 New API 账号分别登录。完整规格和验收进度见 [SAAS_SPEC.md](docs/SAAS_SPEC.md)、[SAAS_STATUS.md](docs/SAAS_STATUS.md)。

使用 Docker Compose v2，从本分支构建包含前端的镜像。下面的命令在本机生成随机凭据；`.env` 已被 Git 和 Docker 构建上下文排除。首次启动前在本机保存管理员密码。

```bash
git clone -b feat/saas-multitenant https://github.com/Sunwuyuan/new-api-sass.git
cd new-api-sass
umask 077
read -r -p '平台管理员邮箱: ' saas_admin_email
cat > .env <<EOF
PLATFORM_ORIGIN=http://localhost:3000
PLATFORM_ADMIN_EMAIL=${saas_admin_email}
PLATFORM_ADMIN_PASSWORD=$(openssl rand -hex 24)
SESSION_SECRET=$(openssl rand -hex 32)
CRYPTO_SECRET=$(openssl rand -hex 32)
DB_PASSWORD=$(openssl rand -hex 32)
REDIS_PASSWORD=$(openssl rand -hex 32)
EOF
docker compose up -d --build
```

Compose 在本机构建此分支，沿用 `calciumion/new-api:latest` 本地标签；`pull_policy: build` 保证应用源码来自当前工作目录。PostgreSQL 与 Redis 仅在容器网络内访问，数据库保存在 `pg_data`，应用文件和日志分别在 `data/`、`logs/`。上线时把 `PLATFORM_ORIGIN` 改为实际 HTTPS Origin（例如 `https://gateway.example.com`，不要包含路径），在反向代理中原样转发 `/platform`、`/t/` 和静态资源，并按实际代理网段配置 `TRUSTED_PROXIES`。健康检查使用 `/platform/api/plans`。

1. 访问 `http://localhost:3000/platform`，用 `.env` 中的管理员账号登录。普通用户通过 `/platform/sign-up` 注册平台账号。
2. 创建空间时填写名称和 slug；新空间默认 Lite，自动创建空间 root。立即保存页面提供的一次性激活链接，在 30 分钟内设置 root 密码，再从 `/t/{slug}/sign-in` 登录。平台密码与 root 密码独立。
3. 进入空间后按原 New API 流程配置自己的渠道、用户、令牌和额度。新 root 的钱包初始额度为 0，使用网关前需在空间用户管理中分配额度；托管套餐不包含上游模型费用。客户端 API Base URL 为 `https://gateway.example.com/t/{slug}/v1`。
4. 平台管理员可在 `/platform/admin/workspaces` 的实例管理页人工开通 Lite / Standard / Pro（1–36 个月）、停用或恢复空间。托管套餐不接在线支付。到期或停用后空间请求会被拒绝；管理员续期开通后恢复。
5. Lite 每个 UTC 自然月最多 1,000 次成功或计费请求、5 个用户（含 root）、20 个令牌和 3 个渠道，强制显示平台页脚。Pro 为 100,000 次、1,000 个用户、10,000 个令牌、100 个渠道，可修改平台页脚。未收费的失败请求释放预留次数；月上限返回 HTTP 429 / `tenant_monthly_limit_exceeded`，人工升级后立即恢复。

平台管理员首次创建后可从部署环境移除 `PLATFORM_ADMIN_EMAIL` 和 `PLATFORM_ADMIN_PASSWORD`；两项必须一起移除。已有管理员不会因启动变量重新设置密码，日常改密使用平台的「修改密码」，并撤销其全部会话。`SESSION_SECRET`、`CRYPTO_SECRET` 必须长期保存，所有应用副本使用相同值。

也可用 Go 直接启动（Go 版本以 `go.mod` 为准，前端用 Bun）。先设置上述平台与密钥环境变量，再执行：

```bash
cd web
bun install --frozen-lockfile
bun run build:check
cd ..
go build -o /tmp/new-api-saas .
# SQLite 示例；数据库目录必须已存在并可写
mkdir -p data
SQL_DSN=local SQLITE_PATH="$PWD/data/saas.db" /tmp/new-api-saas
```

MySQL 使用 `SQL_DSN='用户:密码@tcp(主机:3306)/数据库?charset=utf8mb4&parseTime=true'`，PostgreSQL 使用 `SQL_DSN='postgres://用户:密码@主机:5432/数据库?sslmode=require'`；凭据从部署环境提供。可单独设置 `LOG_SQL_DSN` 为 SQLite/MySQL/PostgreSQL 或 `clickhouse://用户:密码@主机:9000/日志库`，未设置时日志使用主库。连接字符串中的特殊字符须按相应驱动规则编码；Compose 示例生成十六进制密码以免产生 URI 转义问题。

升级独立 New API 数据库前，停止旧服务并备份主库、独立日志库和文件目录。首次启动会迁移业务表的 `tenant_id`、复合唯一键与日志，将旧数据归入 `/t/imported/`，保留原空间账号和密码，并将其关联到首个平台管理员。旧站点地址会更新到新路径，OAuth 与支付回调需在外部服务同步调整；旧浏览器会话需重新登录。迁移期间只启动一个实例；遇到未知自定义唯一索引会拒绝自动迁移，须先显式补齐该索引的租户维度。迁移成功后再次启动验证幂等，再加入其他副本。Lite 会显示强制平台页脚，旧自定义页脚数据保留，开通 Pro 后按能力矩阵使用。

水平扩容时，各副本共享数据库、Redis、密钥与公共 Origin；文件制品使用共享文件目录或配置对象存储。每个副本仍是服务全部租户的单进程，不需要为单个空间启动服务。设置和缓存通过共享调度器定期同步，套餐与用量限制由数据库原子检查。新旧数据库迁移及隔离验收命令、实际版本见 [SAAS_STATUS.md](docs/SAAS_STATUS.md)。

### 平台认证与独立工作台

平台登录和注册分别位于 `/platform/sign-in`、`/platform/sign-up`，与 `/t/{slug}/` 内的 New API 账号、配置和会话相互独立。平台工作台提供空间列表、创建、按实例管理、兑换、套餐比较、用量分析和账号安全；站长后台在 `/platform/admin`。

认证配置由进程启动时读取的环境变量决定，修改后需重启。`/platform/api/status` 仅返回公开开关、品牌和条款地址，不返回客户端密钥或令牌。

| 变量 | 用途 |
| --- | --- |
| `PLATFORM_NAME`、`PLATFORM_LOGO` | 平台显示名称与 Logo；Logo 使用 HTTPS URL 或本站绝对路径 |
| `PLATFORM_REGISTER_ENABLED` | 是否允许创建平台账号，默认 `true` |
| `PLATFORM_PASSWORD_LOGIN_ENABLED` | 密码登录和密码注册开关，默认 `true` |
| `PLATFORM_OAUTH_REGISTER_ENABLED` | 是否允许第三方首次登录创建新账号，默认 `true`，同时受注册总开关控制 |
| `PLATFORM_PASSKEY_ENABLED` | 启用平台 Passkey，默认 `false`；使用 `PLATFORM_ORIGIN` 的域名作为 RP ID |
| `PLATFORM_AGREEMENT_URL`、`PLATFORM_PRIVACY_URL` | 可选的平台条款和隐私政策 HTTPS URL 或本站绝对路径 |
| `PLATFORM_{GITHUB,DISCORD,LINUXDO,OIDC,TELEGRAM}_CLIENT_ID` / `_CLIENT_SECRET` | 各提供方的平台应用凭据；默认在配置凭据后启用，可用对应 `_ENABLED=false` 关闭 |
| `PLATFORM_OIDC_ISSUER`、`PLATFORM_OIDC_NAME` | OIDC discovery issuer 与按钮显示名称 |
| `PLATFORM_WECHAT_ENABLED`、`PLATFORM_WECHAT_SERVER`、`PLATFORM_WECHAT_TOKEN`、`PLATFORM_WECHAT_QRCODE` | 微信开关、验证码服务地址、服务访问令牌和二维码地址 |
| `PLATFORM_CUSTOM_OAUTH_PROVIDERS` | 自定义提供方 JSON 数组，最多 20 个 |

在各提供方后台登记精确回调地址：`{PLATFORM_ORIGIN}/platform/oauth/{provider}`，其中 provider 为 `github`、`discord`、`linuxdo`、`oidc`、`telegram` 或自定义 `slug`。OAuth 使用授权码、S256 PKCE 和绑定浏览器的一次性 state；提供方必须支持相应契约。Telegram 使用 OAuth/OIDC 应用凭据。不要复用空间内设置的 OAuth 回调地址。

自定义提供方必填 `slug`、`name`、`client_id`、`client_secret`；OIDC 配置 `issuer` 和含 `openid` 的 `scopes`，普通 OAuth 配置 `authorization_endpoint`、`token_endpoint`、`userinfo_endpoint`、`subject_path`，可选 `name_path`、`scopes` 与 `token_auth_method`（`client_secret_basic` 或 `client_secret_post`）。凭据通过受保护的部署环境注入，不将包含密钥的 JSON、`.env` 或数据库提交到仓库。

微信服务沿用 `GET /api/wechat/user?code=...`、`Authorization` 令牌和 `{success,data}` 响应契约；服务必须保证验证码最长五分钟有效并单次消费。平台额外执行浏览器绑定、数据库限流和跨副本重放保护。外部邮箱声明不会自动合并平台账号；已有账号须登录并验证身份后主动关联。

在「账号安全」中登记 Passkey；绑定、删除和站长敏感操作要求五分钟内认证。重新认证可使用密码、已登记 Passkey、已关联微信或能返回并验证新 `auth_time` 的 OIDC。普通 OAuth 不作为可强制刷新认证时间的验证方式；仅有该方式时需退出后重新登录，并建议登记 Passkey。关闭密码登录前，先为管理员配置并实际验证可用的替代方式。平台会话保持八小时绝对、三十分钟空闲过期；平台退出不注销外部 IdP 会话。平台 SMTP 密码找回、OTP/MFA 和在线支付不在本阶段提供。

以下保留上游 New API 的部署和功能资料；本分支的 SaaS 启动与平台管理按上节执行。

### 使用 Docker Compose（推荐）

```bash
# 克隆项目
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# 编辑 docker-compose.yml 配置
nano docker-compose.yml

# 启动服务
docker-compose up -d
```

<details>
<summary><strong>使用 Docker 命令</strong></summary>

```bash
# 拉取最新镜像
docker pull calciumion/new-api:latest

# 使用 SQLite（默认）
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest

# 使用 MySQL
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

> **💡 提示：** `-v ./data:/data` 会将数据保存在当前目录的 `data` 文件夹中，你也可以改为绝对路径如 `-v /your/custom/path:/data`

</details>

---

🎉 部署完成后，访问 `http://localhost:3000` 即可使用！

> [!WARNING]
> 将本项目作为面向公众的生成式 AI 服务或 API 转售服务运营时，使用者应先完成备案、内容安全、实名、日志留存、税务、支付和上游授权等合规义务。

📖 更多部署方式请参考 [部署指南](https://docs.newapi.pro/zh/docs/installation)

---

## 📚 文档

<div align="center">

### 📖 [官方文档](https://docs.newapi.pro/zh/docs) | [![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/QuantumNous/new-api)

</div>

**快速导航：**

| 分类 | 链接 |
|------|------|
| 🚀 部署指南 | [安装文档](https://docs.newapi.pro/zh/docs/installation) |
| ⚙️ 环境配置 | [环境变量](https://docs.newapi.pro/zh/docs/installation/config-maintenance/environment-variables) |
| 📡 接口文档 | [API 文档](https://docs.newapi.pro/zh/docs/api) |
| ❓ 常见问题 | [FAQ](https://docs.newapi.pro/zh/docs/support/faq) |
| 💬 社区交流 | [交流渠道](https://docs.newapi.pro/zh/docs/support/community-interaction) |

---

## ✨ 主要特性

> 详细特性请参考 [特性说明](https://docs.newapi.pro/zh/docs/guide/wiki/basic-concepts/features-introduction)

### 🎨 核心功能

| 特性 | 说明 |
|------|------|
| 🎨 全新 UI | 现代化的用户界面设计 |
| 🌍 多语言 | 支持中文、英文、法语、日语 |
| 🔄 数据兼容 | 完全兼容原版 One API 数据库 |
| 📈 数据看板 | 可视化控制台与统计分析 |
| 🔒 权限管理 | 令牌分组、模型限制、用户管理 |

### 💰 授权用量与成本管理

- ✅ 合法授权场景下的内部充值与额度分配（易支付、Stripe）
- ✅ 组织内按次、按量或缓存命中成本核算
- ✅ 支持 OpenAI、Azure、DeepSeek、Claude、Qwen 等模型的缓存计费统计
- ✅ 面向内部管理或企业客户的灵活计费策略配置

### 🔐 授权与安全

- 😈 Discord 授权登录
- 🤖 LinuxDO 授权登录
- 📱 Telegram 授权登录
- 🔑 OIDC 统一认证
- 🔍 Key 查询使用额度（配合 [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool)）

### 🚀 高级功能

**API 格式支持：**
- ⚡ [OpenAI Responses](https://docs.newapi.pro/zh/docs/api/ai-model/chat/openai/create-response)
- ⚡ [OpenAI Realtime API](https://docs.newapi.pro/zh/docs/api/ai-model/realtime/create-realtime-session)（含 Azure）
- ⚡ [Claude Messages](https://docs.newapi.pro/zh/docs/api/ai-model/chat/create-message)
- ⚡ [Google Gemini](https://doc.newapi.pro/api/google-gemini-chat)
- 🔄 [Rerank 模型](https://docs.newapi.pro/zh/docs/api/ai-model/rerank/create-rerank)（Cohere、Jina）

**智能路由：**
- ⚖️ 渠道加权随机
- 🔄 失败自动重试
- 🚦 用户级别模型限流

**格式转换：**
- 🔄 **OpenAI Compatible ⇄ Claude Messages**
- 🔄 **OpenAI Compatible → Google Gemini**
- 🔄 **Google Gemini → OpenAI Compatible** - 仅支持文本，暂不支持函数调用
- 🚧 **OpenAI Compatible ⇄ OpenAI Responses** - 开发中
- 🔄 **思考转内容功能**

**Reasoning Effort 支持：**

<details>
<summary>查看详细配置</summary>

**OpenAI 系列模型：**
- `o3-mini-high` - High reasoning effort
- `o3-mini-medium` - Medium reasoning effort
- `o3-mini-low` - Low reasoning effort
- `gpt-5-high` - High reasoning effort
- `gpt-5-medium` - Medium reasoning effort
- `gpt-5-low` - Low reasoning effort

**Claude 思考模型：**
- `claude-3-7-sonnet-20250219-thinking` - 启用思考模式

**Google Gemini 系列模型：**
- `gemini-2.5-flash-thinking` - 启用思考模式
- `gemini-2.5-flash-nothinking` - 禁用思考模式
- `gemini-2.5-pro-thinking` - 启用思考模式
- `gemini-2.5-pro-thinking-128` - 启用思考模式，并设置思考预算为128tokens
- 也可以直接在 Gemini 模型名称后追加 `-low` / `-medium` / `-high` 来控制思考力度（无需再设置思考预算后缀）

</details>

---

## 🤖 模型支持

> 详情请参考 [接口文档 - 网关接口](https://docs.newapi.pro/zh/docs/api)

| 模型类型 | 说明 | 文档 |
|---------|------|------|
| 🤖 OpenAI-Compatible | OpenAI 兼容模型 | [文档](https://docs.newapi.pro/zh/docs/api/ai-model/chat/openai/createchatcompletion) |
| 🤖 OpenAI Responses | OpenAI Responses 格式 | [文档](https://docs.newapi.pro/zh/docs/api/ai-model/chat/openai/createresponse) |
| 🎨 Midjourney-Proxy | [Midjourney-Proxy(Plus)](https://github.com/novicezk/midjourney-proxy) | [文档](https://doc.newapi.pro/api/midjourney-proxy-image) |
| 🎵 Suno-API | [Suno API](https://github.com/Suno-API/Suno-API) | [文档](https://doc.newapi.pro/api/suno-music) |
| 🔄 Rerank | Cohere、Jina | [文档](https://docs.newapi.pro/zh/docs/api/ai-model/rerank/create-rerank) |
| 💬 Claude | Messages 格式 | [文档](https://docs.newapi.pro/zh/docs/api/ai-model/chat/createmessage) |
| 🌐 Gemini | Google Gemini 格式 | [文档](https://docs.newapi.pro/zh/docs/api/ai-model/chat/gemini/geminirelayv1beta) |
| 🔧 Dify | ChatFlow 模式 | - |
| 🎯 自定义上游 | 支持配置合法授权的上游接口地址 | - |

### 📡 支持的接口

<details>
<summary>查看完整接口列表</summary>

- [聊天接口 (Chat Completions)](https://docs.newapi.pro/zh/docs/api/ai-model/chat/openai/createchatcompletion)
- [响应接口 (Responses)](https://docs.newapi.pro/zh/docs/api/ai-model/chat/openai/createresponse)
- [图像接口 (Image)](https://docs.newapi.pro/zh/docs/api/ai-model/images/openai/post-v1-images-generations)
- [音频接口 (Audio)](https://docs.newapi.pro/zh/docs/api/ai-model/audio/openai/create-transcription)
- [视频接口 (Video)](https://docs.newapi.pro/zh/docs/api/ai-model/videos/sora/createvideo)
- [嵌入接口 (Embeddings)](https://docs.newapi.pro/zh/docs/api/ai-model/embeddings/createembedding)
- [重排序接口 (Rerank)](https://docs.newapi.pro/zh/docs/api/ai-model/rerank/creatererank)
- [实时对话 (Realtime)](https://docs.newapi.pro/zh/docs/api/ai-model/realtime/createrealtimesession)
- [Claude 聊天](https://docs.newapi.pro/zh/docs/api/ai-model/chat/createmessage)
- [Google Gemini 聊天](https://docs.newapi.pro/zh/docs/api/ai-model/chat/gemini/geminirelayv1beta)

</details>

---

## 🚢 部署

> [!TIP]
> **最新版 Docker 镜像：** `calciumion/new-api:latest`

### 📋 部署要求

| 组件 | 要求 |
|------|------|
| **本地数据库** | SQLite（Docker 需挂载 `/data` 目录）|
| **远程数据库** | MySQL ≥ 5.7.8 或 PostgreSQL ≥ 9.6 |
| **容器引擎** | Docker / Docker Compose |
| **系统架构** | 仅支持 64 位系统（amd64 / arm64），不支持 32 位系统 |

### ⚙️ 环境变量配置

<details>
<summary>常用环境变量配置</summary>

| 变量名 | 说明                                                           | 默认值 |
|--------|--------------------------------------------------------------|--------|
| `SESSION_SECRET` | 鉴权签名密钥；所有节点必须保持一致                                           | - |
| `SESSION_COOKIE_SECURE` | `false`/未配置时关闭 refresh/logout OriginGuard 以兼容本地 HTTP 开发代理；`true` 时启用 Secure Cookie 和严格 Origin 校验 | `false` |
| `SESSION_COOKIE_TRUSTED_URL` | Secure 模式必填：允许调用 refresh/logout 的精确 HTTPS Origin，多个用英文逗号分隔；不是 relay CORS 白名单 | - |
| `TRUSTED_PROXIES` | 未配置/留空时信任回环、RFC1918 和 IPv6 ULA 并输出启动告警；`none` 不信任任何代理；显式代理 IP/CIDR 列表完全替代默认值 | `127.0.0.0/8, ::1, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, fc00::/7` |
| `USER_SESSION_ACTIVE_LIMIT` | 单用户最大活跃登录 Session 数 | `50` |
| `USER_SESSION_ISSUANCE_LIMIT` | 单用户在签发窗口内可创建的 Session 总数，包含已撤销 Session | `100` |
| `USER_SESSION_ISSUANCE_WINDOW_SECONDS` | Session 签发计数窗口（秒）；高于 revoked 保留期时自动钳制 | `86400` |
| `USER_SESSION_REVOKED_RETENTION_DAYS` | revoked Session 用于审计和签发计数的保留天数 | `7` |
| `USER_SESSION_HOURLY_ALERT_THRESHOLD` | 全局每小时 Session 签发告警阈值；只告警，不拒绝登录 | `5000` |
| `CRYPTO_SECRET` | 缓存键 HMAC 密钥；共享 Redis 的节点必须使用相同有效值 | 默认跟随 `SESSION_SECRET` |
| `SQL_DSN` | 数据库连接字符串                                                     | - |
| `REDIS_CONN_STRING` | Redis 连接字符串                                                  | - |
| `STREAMING_TIMEOUT` | 流式超时时间（秒）                                                    | `300` |
| `STREAM_SCANNER_MAX_BUFFER_MB` | 流式扫描器单行最大缓冲（MB），图像生成等超大 `data:` 片段（如 4K 图片 base64）需适当调大 | `64` |
| `MAX_REQUEST_BODY_MB` | 请求体最大大小（MB，**解压后**计；防止超大请求/zip bomb 导致内存暴涨），超过将返回 `413` | `32` |
| `AZURE_DEFAULT_API_VERSION` | Azure API 版本                                                 | `2025-04-01-preview` |
| `ERROR_LOG_ENABLED` | 错误日志开关                                                       | `false` |
| `PYROSCOPE_URL` | Pyroscope 服务地址                                            | - |
| `PYROSCOPE_APP_NAME` | Pyroscope 应用名                                        | `new-api` |
| `PYROSCOPE_BASIC_AUTH_USER` | Pyroscope Basic Auth 用户名                        | - |
| `PYROSCOPE_BASIC_AUTH_PASSWORD` | Pyroscope Basic Auth 密码                  | - |
| `PYROSCOPE_MUTEX_RATE` | Pyroscope mutex 采样率                               | `5` |
| `PYROSCOPE_BLOCK_RATE` | Pyroscope block 采样率                               | `5` |
| `HOSTNAME` | Pyroscope 标签里的主机名                                          | `new-api` |

📖 **完整配置：** [环境变量文档](https://docs.newapi.pro/zh/docs/installation/config-maintenance/environment-variables)

</details>

### 🔧 部署方式

<details>
<summary><strong>方式 1：Docker Compose（推荐）</strong></summary>

```bash
# 克隆项目
git clone https://github.com/QuantumNous/new-api.git
cd new-api

# 编辑配置
nano docker-compose.yml

# 启动服务
docker-compose up -d
```

</details>

<details>
<summary><strong>方式 2：Docker 命令</strong></summary>

**使用 SQLite：**
```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

**使用 MySQL：**
```bash
docker run --name new-api -d --restart always \
  -p 3000:3000 \
  -e SQL_DSN="root:123456@tcp(localhost:3306)/oneapi" \
  -e TZ=Asia/Shanghai \
  -v ./data:/data \
  calciumion/new-api:latest
```

> **💡 路径说明：**
> - `./data:/data` - 相对路径，数据保存在当前目录的 data 文件夹
> - 也可使用绝对路径，如：`/your/custom/path:/data`

</details>

<details>
<summary><strong>方式 3：宝塔面板</strong></summary>

1. 安装宝塔面板（≥ 9.2.0 版本）
2. 在应用商店搜索 **New-API**
3. 一键安装

📖 [图文教程](./docs/installation/BT.md)

</details>

### ⚠️ 多机部署注意事项

> [!WARNING]
> - 所有节点必须使用同一个主数据库，并设置相同的 `SESSION_SECRET`；否则 Access Token、Refresh 会话和临时鉴权流程无法一致校验。
> - 连接同一个 Redis 的节点还必须设置相同的 `CRYPTO_SECRET`，否则节点生成的缓存键摘要不一致，无法正确共享缓存。

登录 Session 和单用户活跃数/签发数限制均以数据库为权威。Redis 中的 Session 仅为短期缓存，TTL 跟随 `SYNC_FREQUENCY`（默认 60 秒），且不会超过 Session 的剩余寿命。

| Redis 拓扑 | Session 状态传播 | 限流语义 |
| --- | --- | --- |
| 所有节点共享 Redis | 撤销和版本发布通常即时传播 | Redis 限流额度在节点间共享 |
| 每个节点使用独立 Redis | 最迟在有效 `SYNC_FREQUENCY` 内回源数据库收敛；版本轮换后，新 Token 在持有旧缓存的节点上可能短暂返回 401 | 每个节点独立计数，集群总额度最坏约为单节点阈值乘以节点数 |
| 不使用 Redis | 每次 Session 校验直接读取数据库 | 各节点使用独立的内存限流额度 |

缩短 `SYNC_FREQUENCY` 可减小独立 Redis 的陈旧窗口，但每个活跃 SID 在每个节点上会按该 TTL 增加一次数据库主键点查。上述保证只让 Session 鉴权在不同拓扑下保持有界陈旧；限流和其他 Redis 控制面缓存仍受拓扑影响。

Token、Origin 校验和 PAT 契约见[用户鉴权与登录会话](./docs/authentication.md)。

### 🔄 渠道重试与缓存

**重试配置：** `设置 → 运营设置 → 通用设置 → 失败重试次数`

**缓存配置：**
- `REDIS_CONN_STRING`：Redis 缓存（推荐）
- `MEMORY_CACHE_ENABLED`：内存缓存

---

## 🔗 相关项目

### 上游项目

| 项目 | 说明 |
|------|------|
| [One API](https://github.com/songquanpeng/one-api) | 原版项目基础 |
| [Midjourney-Proxy](https://github.com/novicezk/midjourney-proxy) | Midjourney 接口支持 |

### 配套工具

| 项目 | 说明 |
|------|------|
| [new-api-key-tool](https://github.com/Calcium-Ion/new-api-key-tool) | Key 额度查询工具 |
| [new-api-horizon](https://github.com/Calcium-Ion/new-api-horizon) | New API 高性能优化版 |

---

## 💬 帮助支持

### 📖 文档资源

| 资源 | 链接 |
|------|------|
| 📘 常见问题 | [FAQ](https://docs.newapi.pro/zh/docs/support/faq) |
| 💬 社区交流 | [交流渠道](https://docs.newapi.pro/zh/docs/support/community-interaction) |
| 🐛 反馈问题 | [问题反馈](https://docs.newapi.pro/zh/docs/support/feedback-issues) |
| 📚 完整文档 | [官方文档](https://docs.newapi.pro/zh/docs) |

### 🤝 贡献指南

欢迎各种形式的贡献！

- 🐛 报告 Bug
- 💡 提出新功能
- 📝 改进文档
- 🔧 提交代码

---

## 📜 许可证

本项目采用 [GNU Affero 通用公共许可证 v3.0 (AGPLv3)](./LICENSE) 授权。

本项目为开源项目，在 [One API](https://github.com/songquanpeng/one-api)（MIT 许可证）的基础上进行二次开发。

如果您所在的组织政策不允许使用 AGPLv3 许可的软件，或您希望规避 AGPLv3 的开源义务，请发送邮件至：[support@quantumnous.com](mailto:support@quantumnous.com)

---

## 🌟 Star History

<div align="center">

[![Star History Chart](https://api.star-history.com/svg?repos=Calcium-Ion/new-api&type=Date)](https://star-history.com/#Calcium-Ion/new-api&Date)

</div>

---

<div align="center">

### 💖 感谢使用 New API

如果这个项目对你有帮助，欢迎给我们一个 ⭐️ Star！

**[官方文档](https://docs.newapi.pro/zh/docs)** • **[问题反馈](https://github.com/Calcium-Ion/new-api/issues)** • **[最新发布](https://github.com/Calcium-Ion/new-api/releases)**

<sub>Built with ❤️ by QuantumNous</sub>

</div>
