# DriftGuard

![CI](https://github.com/Cipher-08/driftguard/actions/workflows/ci.yml/badge.svg)

**Unified multi-cloud drift detection & compliance intelligence for mid-market teams.**

DriftGuard connects to your cloud accounts, snapshots live infrastructure state, detects
how it has **drifted** from what your code declares, flags **compliance** violations
(CIS / SOC2-style), and can generate a **Terraform remediation patch** — then open a pull
request back to your IaC repo. One dashboard. No PhD in Terraform required.

---

## Why

> *"What is my actual infra state right now, how far has it drifted from code, and is it compliant — all in one place?"*

Big players (Wiz, Prisma, Spacelift) are built for enterprise. DriftGuard targets the
underserved 50–500 engineer segment and is **fully self-hostable on free/open-source tooling**.

---

## Features

- 🔌 **Multi-cloud collectors** — AWS (EC2, S3, IAM) and GCP (Compute, Cloud Storage) today.
- 🔍 **Drift engine** — field-level diff of live vs. declared state, severity-scored, deduped to one open record per resource.
- 🛡️ **Compliance engine** — built-in CIS/SOC2-style rules (public buckets, missing encryption, public IPs, …). Zero external dependencies.
- ✨ **AI remediation (free & pluggable)** — generates a Terraform patch using **Groq**, **Google Gemini**, or local **Ollama**. No paid key required; the rest of the app works without any AI configured.
- 🔁 **One-click PRs** — push the generated patch to your GitHub repo as a pull request.
- 📊 **Dashboard** — drift-by-severity, compliance posture, resource inventory, on-demand scans.
- 🔐 **Multi-tenant** — org + JWT auth, per-org cloud credentials.

---

## Architecture

| Layer            | Tech                                             |
| ---------------- | ------------------------------------------------ |
| API + collectors | Go 1.25, Gin                                     |
| Drift/compliance | Native Go engine                                 |
| AI remediation   | Pluggable: Groq / Gemini / Ollama (all free)     |
| Storage          | PostgreSQL, Redis                                |
| Frontend         | React + Vite + Tailwind + React Query            |
| Infra            | Docker Compose                                   |

```
cmd/api            → entrypoint, wiring, scheduler
internal/
  api/             → routes, handlers, JWT middleware
  collectors/aws   → live AWS state
  collectors/gcp   → live GCP state (REST + service-account auth)
  engine/          → drift detection + severity scoring
  compliance/      → built-in policy rules
  llm/             → provider-agnostic remediation (groq/gemini/ollama)
  remediation/     → GitHub PR creation
  db/              → connection pool + embedded migrations
frontend/          → React dashboard
```

---

## Quick start (Docker — one command)

```bash
git clone https://github.com/Cipher-08/driftguard.git
cd driftguard
cp .env.example .env        # tweak secrets / add an AI key if you want remediation
docker compose up --build
```

- Dashboard → http://localhost:3000
- API/health → http://localhost:8080/health

Then **Register** an account, go to **Cloud Accounts**, and connect AWS or GCP read-only
credentials. DriftGuard scans immediately and every 15 minutes after.

## Local development

```bash
# Postgres + Redis only
docker compose up -d postgres redis

# Backend
go run ./cmd/api

# Frontend (separate terminal)
cd frontend && npm install && npm run dev   # http://localhost:3000 (proxies /api → :8080)
```

Run the tests:

```bash
go test ./...
```

---

## Enabling AI remediation (optional, free)

Set **one** of these in `.env` — DriftGuard auto-detects it:

| Provider | Env vars | Where to get it |
| -------- | -------- | --------------- |
| Groq (recommended) | `GROQ_API_KEY` | https://console.groq.com (free) |
| Google Gemini | `GEMINI_API_KEY` | https://aistudio.google.com/app/apikey (free tier) |
| Ollama (local) | `OLLAMA_HOST=http://localhost:11434` | https://ollama.com (fully local) |

To open remediation PRs, also set `GITHUB_TOKEN` (a PAT with `repo` scope).

`GET /health` reports which provider is active (`ai_provider`) and whether GitHub PRs are ready.

---

## API overview

| Method | Path | Description |
| ------ | ---- | ----------- |
| POST | `/api/v1/auth/register` | Create org + admin user |
| POST | `/api/v1/auth/login` | Get a JWT |
| GET  | `/api/v1/summary` | Dashboard counts (drift + compliance) |
| GET/POST/DELETE | `/api/v1/credentials` | Manage cloud accounts |
| POST | `/api/v1/scan` | Trigger an on-demand scan |
| GET  | `/api/v1/resources` | Resource inventory |
| GET  | `/api/v1/drifts` | List open drifts |
| GET  | `/api/v1/drifts/:id` | Drift detail + violations + remediations |
| PATCH| `/api/v1/drifts/:id/resolve` | Resolve a drift |
| POST | `/api/v1/drifts/:id/remediation` | Generate an AI Terraform patch |
| POST | `/api/v1/drifts/:id/remediation/:rem_id/pr` | Open a GitHub PR with the patch |

---

## Roadmap

- [x] AWS collector (EC2, S3, IAM)
- [x] GCP collector (Compute, Cloud Storage)
- [x] Drift detection engine (field-level, severity-scored)
- [x] Compliance engine (CIS/SOC2 built-in rules)
- [x] AI remediation (free, pluggable) + GitHub PRs
- [x] Dashboard, auth, multi-tenant, Docker Compose
- [ ] **IaC declared-state ingestion** (clone repo + parse Terraform → populate `declared_state`)
- [ ] Azure collector
- [ ] CloudTrail / audit-log attribution ("who changed what")
- [ ] Slack/email alerting

> **Note on drift vs. declared state:** the `iac_repos` table and `declared_state` column
> exist, but declared-state ingestion is the next milestone. Until a repo is connected,
> resources are reported as **unmanaged** drift and evaluated for compliance.

---

© 2026 Saksham Awasthi.
