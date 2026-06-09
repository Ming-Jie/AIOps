# AIOps

[English](README-en.md)

---

## 中文

### 简介

AIOps 是一个开源的 AI Agent 平台，支持 Agent 管理、技能系统、MCP 协议、知识库 RAG、工作流、IM 机器人、聊天对话与 RBAC 权限控制。

- **后端**：Go (Hertz) + PostgreSQL (pgvector)
- **前端**：可选嵌入的 Vue3 Web UI
- **桌面端**：可选 Tauri 客户端（见 `aiops-desktop/`）

### 功能特性

| 功能 | 说明 |
|------|------|
| Agent 管理 | 创建、配置，绑定 Skill / MCP / 知识库，支持多种执行模式（默认、ReAct、计划执行） |
| 技能系统 | Skill 封装与动态加载 |
| MCP 协议 | 按 Agent 绑定 MCP 工具 |
| 知识库 | 文档上传与 HTTPS 链接批量导入（OpenViking 向量库）、预览、语义检索、Agent RAG |
| 工作流 | 可视化图工作流、顺序多 Agent 对话流 |
| 聊天对话 | 多会话、流式输出、长期记忆、附件 |
| IM 机器人 | 飞书 / Lark、钉钉、Telegram — Agent 在 IM 渠道自动回复 |
| SSO 登录 | 飞书、钉钉、企业微信、Telegram OAuth |
| 定时任务 | Cron 式调度 Agent 执行 |
| RBAC | 用户、角色、权限控制 |
| 长期记忆 | pgvector 向量存储与检索 |
| 沙箱执行 | Docker 沙箱代码执行（需挂载 `/var/run/docker.sock`） |
| 可观测性 | Langfuse 链路追踪（可选） |

### 知识库（可选）

依赖 PostgreSQL + OpenViking。

1. 在 `.env` 中启用：
   ```bash
   OPENVIKING_ENABLED=true
   OPENVIKING_URL=http://127.0.0.1:1933
   OPENVIKING_API_KEY=<与 deploy/openviking/ov.conf 中一致>
   ```
2. 使用 Compose profile 启动 OpenViking：
   ```bash
   docker compose --profile openviking up -d
   ```
   也可将 `OPENVIKING_URL` 指向外部 OpenViking 服务。

**能力概览：**
- 支持 PDF、Markdown、Office、HTML 等格式上传；OpenViking 异步建立向量索引
- UI 弹窗支持单个或多个 HTTPS 链接批量导入
- 原文预览（本地存储）+ 知识库详情页语义检索
- Agent 绑定知识库后，Web 聊天、IM 机器人、定时任务均会自动 RAG 检索

**存储说明：** 原件保存在 `UPLOAD_DIR`（本地默认 `./uploads`，Docker 内 `/app/data/uploads`）。生产环境可在 `.env` 设置 `UPLOAD_HOST_PATH` 将宿主机目录挂进容器。向量数据在 OpenViking 卷中 — 需同时备份 Postgres、`UPLOAD_DIR` 与 OpenViking 数据。

### 环境要求

| 组件 | 最低版本 |
|------|----------|
| Docker | 20.10+ |
| Docker Compose | 2.0+ |
| PostgreSQL | 15+ (带 pgvector) |
| Node.js | 22+ (前端开发) |
| Go | 1.24+ (后端开发) |
| OpenViking | 可选，知识库功能需要 |

### 快速开始

#### Docker 部署

默认包含 PostgreSQL（pgvector），直接启动即可。

```bash
cp .env.example .env
# 编辑 .env，必填：JWT_SECRET_KEY、OPENAI_API_KEY、ADMIN_DEFAULT_PASSWORD
docker compose up -d
```

启用知识库 + 内置 OpenViking：

```bash
# 先在 .env 中设置 OPENVIKING_ENABLED=true 和 OPENVIKING_API_KEY
docker compose --profile openviking up -d
```

若改用外部 PostgreSQL，在 `.env` 中设置 `DATABASE_URL`（默认仍会启动 compose 内的 `postgres` 服务，高级场景可自行用 override 禁用）。

**访问：**
- Web: http://localhost:8080
- API: http://localhost:8080/api/v1
- Swagger: http://localhost:8080/swagger

#### 本地开发

```bash
# 后端
cp .env.example .env
go run ./cmd/server

# 前端（另开终端）
cd ui && npm install && npm run dev
```

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `SERVER_PORT` | `8080` | HTTP 端口 |
| `JWT_SECRET_KEY` | - | **必填** JWT 密钥 |
| `MODEL_TYPE` | `openai` | `openai` / `ark` |
| `OPENAI_API_KEY` | - | OpenAI 密钥 |
| `DATABASE_URL` | - | PostgreSQL 连接串 |
| `MEMORY_PROVIDER` | `pgvector` | 长期记忆：`pgvector` / `none` |
| `UPLOAD_DIR` | `uploads` | 聊天附件与知识库原件目录（需持久化） |
| `UPLOAD_HOST_PATH` | - | Docker Compose：宿主机路径挂载到 `/app/data/uploads` |
| `OPENVIKING_ENABLED` | `false` | 启用知识库功能 |
| `OPENVIKING_URL` | `http://127.0.0.1:1933` | OpenViking HTTP 地址 |
| `OPENVIKING_API_KEY` | - | OpenViking API Key |
| `AUTH_TYPE` | `password` | `password` / SSO：`lark`、`dingtalk`、`wecom`、`telegram` |
| `LANGFUSE_*` | - | 可选，LLM 可观测性 |

完整变量列表见 `.env.example`。

### 使用指南

| 操作 | UI 入口 |
|------|---------|
| 创建知识库 | **知识库** → 新建 → 上传文件或 **链接导入**（每行一个 HTTPS 链接） |
| Agent 绑定知识库 | **智能体** → 编辑 → 选择知识库（`kb_ids`） |
| 测试检索 | **知识库** → 进入详情 → **检索** 标签页 |
| 预览文档 | 文档列表中点击文件名（侧栏抽屉） |
| 图工作流 | **工作流** → 可视化编辑器（Agent / 知识库 / 工具节点） |
| IM 机器人 | **智能体** → 启用 IM（飞书 / 钉钉 / Telegram）并填写应用凭证 |
| 定时任务 | **定时任务** → Cron + 目标 Agent |

**Agent RAG 流程：** 用户消息 → 检索已绑定知识库（OpenViking）→ 注入 top 片段 → LLM 生成回复。Web 聊天、IM 机器人、定时任务均走同一套检索（不仅限于 HTTP `/chat`）。

**OpenViking 配置：** 使用 Compose 内置服务时，`deploy/openviking/ov.conf` 中的 `root_api_key` 需与 `.env` 的 `OPENVIKING_API_KEY` 一致；`providers.embedding`（及可选 `vlm`）需配置有效 API Key，否则文档会一直停在「索引中」。

### 目录结构

```
cmd/server/          # 主程序入口
internal/
  agent/             # Runtime、ReAct、工具（MCP + 内置 Skill）
  service/           # 聊天、知识库、工作流
  openviking/        # OpenViking HTTP 客户端
  controller/        # REST API
  dingtalkbot/       # 钉钉机器人
  larkbot/           # 飞书机器人
  telegrambot/       # Telegram 机器人
ui/                  # Vue 3 + Quasar 前端
skills/              # SKILL.md 技能定义
deploy/openviking/   # Compose profile 用的 ov.conf
docker-compose.yml
```

### 常见问题

| 现象 | 排查 |
|------|------|
| 文档一直 **索引中** | OpenViking 是否可达；`ov.conf` 中 embedding API Key；日志是否有 `kb: document indexed` |
| Agent 不用知识库 | 是否保存了 `kb_ids`；`OPENVIKING_ENABLED=true`；日志是否有 `knowledge bases bound for agent`、`knowledge base retrieval started` |
| IM 机器人无 RAG | 同上 — IM 走 Runtime 检索，每条消息应出现 retrieval 相关日志 |
| 重启后上传文件丢失 | 持久化 `UPLOAD_DIR`，或 Compose 中配置 `UPLOAD_HOST_PATH` |
| 日志 `kb_retrieval_enabled=false` | 知识库未初始化（需 Postgres + `OPENVIKING_ENABLED`） |

关键日志：`knowledge base feature enabled`、`knowledge bases bound for agent`、`knowledge base retrieval completed`（含 `hit_count`）。

### 技术栈

- **后端**: Go, Hertz, GORM, pgvector, OpenViking（可选）
- **前端**: Vue 3, Quasar, TypeScript
- **桌面端**: Tauri, React
- **数据库**: PostgreSQL 15+, pgvector
- **LLM**: OpenAI 兼容 API, 火山方舟

### 许可证

Apache License 2.0
