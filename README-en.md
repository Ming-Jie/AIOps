# AIOps

[中文](README.md)

---

## English

### Introduction

AIOps is an open-source AI Agent platform with Agent management, Skill system, MCP protocol, knowledge-base RAG, workflows, IM bots, chat, and RBAC.

- **Backend**: Go (Hertz) + PostgreSQL (pgvector)
- **Frontend**: Optional embedded Vue3 Web UI
- **Desktop**: Optional Tauri desktop client (see `aiops-desktop/`)

### Features

| Feature | Description |
|---------|-------------|
| Agent Management | Create, configure, bind skills / MCP / knowledge bases, execution modes (default, ReAct, plan-execute) |
| Skill System | Skill encapsulation and dynamic loading |
| MCP Protocol | Model Context Protocol tool binding per agent |
| Knowledge Base | Document upload & HTTPS URL import (OpenViking vector store), preview, semantic search, agent RAG |
| Workflows | Visual graph workflows and sequential multi-agent chat flows |
| Chat | Multi-session, streaming, long-term memory, attachments |
| IM Bots | Lark / Feishu, DingTalk, Telegram — agents reply in IM channels |
| SSO | OAuth login via Lark, DingTalk, WeCom, Telegram |
| Schedules | Cron-style scheduled agent runs |
| RBAC | Users, roles, permissions |
| Long-term Memory | pgvector-backed retrieval |
| Sandbox Execution | Docker sandbox for code execution (requires `/var/run/docker.sock`) |
| Observability | Langfuse tracing (optional) |

### Knowledge Base (optional)

Requires PostgreSQL + OpenViking.

1. Enable in `.env`:
   ```bash
   OPENVIKING_ENABLED=true
   OPENVIKING_URL=http://127.0.0.1:1933
   OPENVIKING_API_KEY=<matches deploy/openviking/ov.conf>
   ```
2. Start OpenViking with Compose profile:
   ```bash
   docker compose --profile openviking up -d
   ```
   Or point `OPENVIKING_URL` at an external OpenViking instance.

**What you get:**
- Upload PDF, Markdown, Office, HTML, etc.; async indexing via OpenViking
- Import one or many HTTPS document URLs from the UI dialog
- Preview originals from local storage; semantic search in the KB detail page
- Bind knowledge bases to agents — RAG runs on web chat, IM bots, and scheduled jobs

**Storage:** Original files live under `UPLOAD_DIR` (default `./uploads`, Docker `/app/data/uploads`). Use `UPLOAD_HOST_PATH` in `.env` to bind-mount host disk in Compose. Vector data is in the OpenViking volume — back up Postgres, `UPLOAD_DIR`, and OpenViking data together.

### Requirements

| Component | Minimum Version |
|-----------|-----------------|
| Docker | 20.10+ |
| Docker Compose | 2.0+ |
| PostgreSQL | 15+ (with pgvector) |
| Node.js | 22+ (frontend dev) |
| Go | 1.24+ (backend dev) |
| OpenViking | optional, for knowledge base |

### Quick Start

#### Docker Deployment

Compose includes PostgreSQL (pgvector) by default.

```bash
cp .env.example .env
# Edit .env — required: JWT_SECRET_KEY, OPENAI_API_KEY, ADMIN_DEFAULT_PASSWORD
docker compose up -d
```

Knowledge base + bundled OpenViking:

```bash
# Set OPENVIKING_ENABLED=true and OPENVIKING_API_KEY in .env first
docker compose --profile openviking up -d
```

To use an external PostgreSQL instead, set `DATABASE_URL` in `.env` (the stack still starts the bundled `postgres` service unless you adjust compose with an override).

**Access:**
- Web: http://localhost:8080
- API: http://localhost:8080/api/v1
- Swagger: http://localhost:8080/swagger

#### Local Development

```bash
# Backend
cp .env.example .env
go run ./cmd/server

# Frontend (separate terminal)
cd ui && npm install && npm run dev
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP port |
| `JWT_SECRET_KEY` | - | **Required** JWT secret |
| `MODEL_TYPE` | `openai` | `openai` / `ark` |
| `OPENAI_API_KEY` | - | OpenAI API key |
| `DATABASE_URL` | - | PostgreSQL connection string |
| `MEMORY_PROVIDER` | `pgvector` | Memory: `pgvector` / `none` |
| `UPLOAD_DIR` | `uploads` | Chat attachments & KB originals (persist this path) |
| `UPLOAD_HOST_PATH` | - | Docker Compose: host path bind-mounted to `/app/data/uploads` |
| `OPENVIKING_ENABLED` | `false` | Enable knowledge base feature |
| `OPENVIKING_URL` | `http://127.0.0.1:1933` | OpenViking HTTP base URL |
| `OPENVIKING_API_KEY` | - | OpenViking API key |
| `AUTH_TYPE` | `password` | `password` / SSO: `lark`, `dingtalk`, `wecom`, `telegram` |
| `LANGFUSE_*` | - | Optional LLM observability |

See `.env.example` for the full variable list.

### Usage Guide

| Task | Where in UI |
|------|----------------|
| Create knowledge base | **Knowledge** → New KB → upload files or **Import from URL** (one HTTPS link per line) |
| Bind KB to agent | **Agents** → edit agent → select knowledge bases (`kb_ids`) |
| Test retrieval | **Knowledge** → open KB → **Search** tab |
| Preview document | Click filename in document list (side drawer) |
| Graph workflow | **Workflows** → visual editor (agent / knowledge / tool nodes) |
| IM bot | **Agents** → enable IM (Lark / DingTalk / Telegram) and fill app credentials |
| Scheduled runs | **Schedules** → cron + target agent |

**Agent RAG flow:** User message → bound KBs searched via OpenViking → top chunks injected into context → LLM reply. Works on web chat, IM bots, and scheduled jobs (not only HTTP `/chat`).

**OpenViking config:** When using the bundled service, set `root_api_key` in `deploy/openviking/ov.conf` to match `OPENVIKING_API_KEY`, and configure `providers.embedding` (and optionally `vlm`) API keys — indexing fails without a working embedding provider.

### Project Layout

```
cmd/server/          # Main API entry
internal/
  agent/             # Runtime, ReAct, tools (MCP + builtin skills)
  service/           # Chat, knowledge base, workflows
  openviking/        # OpenViking HTTP client
  controller/        # REST handlers
  dingtalkbot/       # DingTalk bot
  larkbot/           # Lark / Feishu bot
  telegrambot/       # Telegram bot
ui/                  # Vue 3 + Quasar SPA
skills/              # SKILL.md skill definitions
deploy/openviking/   # OpenViking ov.conf for Compose profile
docker-compose.yml
```

### Troubleshooting

| Symptom | Check |
|---------|--------|
| KB stuck on **indexing** | OpenViking reachable; embedding API key in `ov.conf`; server logs `kb: indexing started` / `document indexed` |
| Agent ignores KB | Agent has `kb_ids` saved; `OPENVIKING_ENABLED=true`; logs show `knowledge bases bound for agent` and `knowledge base retrieval started` |
| IM bot no RAG | Same as above — IM uses Runtime RAG (look for retrieval logs on each message) |
| Upload lost after restart | Persist `UPLOAD_DIR` / set `UPLOAD_HOST_PATH` in Compose |
| `kb_retrieval_enabled=false` in logs | KB service not initialized (needs Postgres + `OPENVIKING_ENABLED`) |

Useful log lines: `knowledge base feature enabled`, `knowledge bases bound for agent`, `knowledge base retrieval completed` (with `hit_count`).

### Tech Stack

- **Backend**: Go, Hertz, GORM, pgvector, OpenViking (optional)
- **Frontend**: Vue 3, Quasar, TypeScript
- **Desktop**: Tauri, React
- **Database**: PostgreSQL 15+, pgvector
- **LLM**: OpenAI-compatible API, Volcengine Ark

### License

Apache License 2.0
