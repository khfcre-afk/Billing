# Pterodactyl Billing Platform

Self-hosted billing platform for Pterodactyl Panel. Users top up via **promo codes** (payment gateways intentionally disabled), purchase server tariffs, and have their Pterodactyl servers provisioned automatically.

## Stack

- **Backend:** Go 1.23, [chi](https://github.com/go-chi/chi), PostgreSQL, Redis, goose migrations, zerolog, JWT auth.
- **Frontend:** React 18 + TypeScript + Vite, TanStack Query, Zustand, Tailwind CSS, shadcn-style UI, Framer Motion.
- **Fonts:** Space Grotesk (headings) + Inter (body), loaded via `@fontsource`.
- **Infra:** Docker, docker-compose, GitHub Actions CI.

## Features

- Promo-code only billing (deposit / tariff-grant / percentage-off).
- Automatic Pterodactyl user + server provisioning on purchase.
- Daily cron charges; auto-suspend on zero balance.
- Admin CRUD for promos and tariffs.
- OpenAPI 3.1 spec served at `/docs`.

## Quickstart

```bash
cp .env.example .env
make up
# backend:   http://localhost:8080
# frontend:  http://localhost:5173
# swagger:   http://localhost:8080/docs
```

## Project layout

```
backend/   Go monolith (cmd/ + internal/)
frontend/  React SPA (Vite)
docker-compose.yml
```

See `backend/README.md` and `frontend/README.md` for module-level details.

## Environment variables

See [`.env.example`](./.env.example). Notable flags:

- `PAYMENTS_DISABLED=true` — hard-disables all money-in flows. Promo codes are the only way to grant balance.
- `PTERO_*` — Pterodactyl Panel connection settings.

## License

MIT.
