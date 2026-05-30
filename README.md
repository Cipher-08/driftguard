
# DriftGuard

![CI](https://github.com/Cipher-08/driftguard/actions/workflows/ci.yml/badge.svg)

DriftGuard is a modern, unified dashboard for cloud drift detection and compliance monitoring. Connect your AWS, GCP, or Azure accounts and visualize resource drifts, compliance status, and remediation options—all in one place.

---

## 🚀 Features
- **Secure Cloud Onboarding:** Connect your GCP (and soon AWS/Azure) accounts with a few clicks.
- **Unified Dashboard:** See drift and compliance summaries at a glance.
- **Resource & Drift Explorer:** Browse all detected resources and drifts.
- **Modern UI:** Built with React, Vite, and Tailwind CSS for a fast, beautiful experience.
- **Extensible Backend:** Go, Gin, and Postgres for performance and scalability.

---

## 🛠️ Getting Started

### Prerequisites
- Go 1.22+
- Node.js 20+
- Postgres (for backend DB)

### Setup
```bash
# Clone the repo
git clone https://github.com/Cipher-08/driftguard.git
cd driftguard

# Start backend (in one terminal)
go run cmd/api/main.go

# Start frontend (in another terminal)
cd frontend
npm install
npm run dev
```

### Usage
1. Register/login via the UI
2. Connect your GCP account via the "Connect GCP" button
3. View resources, drifts, and compliance on the dashboard

---

## 📦 Project Structure

- `frontend/` — React app (UI)
- `internal/` — Go backend (API, collectors, models)
- `cmd/api/` — Backend entrypoint
- `.github/workflows/` — CI/CD workflows

---

## 🧩 Status & Roadmap
- [x] GCP credential onboarding UI
- [x] Backend API for credential storage
- [ ] GCP resource collector (coming soon)
- [ ] AWS/Azure support
- [ ] Compliance & remediation automation

---

## 🤝 Contributing
PRs and issues are welcome! Please open an issue to discuss major changes.

---

© 2026 Saksham Awasthi. All rights reserved.
