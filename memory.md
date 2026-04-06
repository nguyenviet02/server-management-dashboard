# Memory — ServerDash Development Notes

> **Purpose**: This file stores cross-AI IDE project memory. If you are Claude Code, Gemini Code, Cursor, Windsurf, or another AI coding tool, read this before starting work to get historical context.
>
> **Token optimization**: Keep this file compact and avoid redundancy. Read referenced documents only when needed.

---

## Current project status

- **Version**: See `/VERSION` (single source of truth)
- **Phase**: Pro edition is complete (12 plugins + security hardening)
- **Build status**: `go build` + `npm run build` pass, `go test ./...` passes
- **Development style**: AI vibe coding
- **Language**: Project docs and comments have historically been Chinese-heavy; code is primarily English

## Document index (read on demand)

| Document | Content | When to read |
|------|------|----------|
| `agents.md` | Architecture, plugin system, directory layout, CoreAPI, security validation | **Read before modifying code** |
| `README.md` | Product features, install guide, configuration | When learning product behavior |
| `docs/install.md` | Detailed install and usage guide | Deployment and operations work |

## Architecture in one sentence

Go (Gin) + React 19 (Vite 7) + 12 compile-time registered plugins managing Caddy reverse proxying. Host DB → `RenderCaddyfile()` → atomic write → `caddy reload` with automatic rollback on failure. Plugins access core functionality through CoreAPI (150+ methods). See `agents.md` for details.

## Development timeline

```text
2025-12    Phase 0    Project initialization, basic Host CRUD + Caddyfile generation
2026-02    Phase 1-4  Core improvements: TLS, DNS providers, multiple host types, Caddyfile editor
2026-02-23 v0.5.1     Security hardening, Altcha PoW, theme system
2026-03-01 v1.0.0     Pro edition: plugin framework + all 12 plugins completed
2026-03-20 v0.9.5     Full security review hardening (100+ security/reliability fixes)
```

## Important conventions

### Version management
- Only update the `VERSION` file and `web/package.json`
- Go version is injected via `-ldflags "-X main.Version=..."`

### Data model
- Boolean fields always use `*bool` pointers (avoid GORM zero-value traps)
- Host has 4 `host_type` values: proxy / redirect / static / php
- Host has 5 `tls_mode` values: auto / dns / wildcard / custom / off
- Plugin tables use the `plugin_{id}_*` prefix

### Caddyfile rendering
- `RenderCaddyfile(hosts, cfg, dnsProviders)` accepts 3 parameters
- All user input must pass `ValidateCaddyValue()` / `ValidateUpstream()` / `ValidateDomain()`
- DNS credentials must pass `safeDnsValue()` validation
- BasicAuth usernames must pass `ValidateCaddyValue()`
- Custom directives are protected against injection via per-character brace-depth tracking

### Plugin development
- Plugin interface: `Metadata()` / `Init(ctx)` / `Start()` / `Stop()`
- Register routes in `Init()`: `ctx.Router` (authenticated users) / `ctx.AdminRouter` (admin only)
- When adding a CoreAPI method, update `types.go`, `coreapi.go`, and the two test stubs together
- Frontend plugin routes are declared via `FrontendManifest` and injected into the sidebar automatically

### Frontend
- Component library: Radix UI Themes (not shadcn)
- State management: Zustand
- Internationalization: react-i18next (`en.json` / `vi.json`)
- Plugin name/description translations: `plugins.names.*` / `plugins.descriptions.*`

### Security
- Minimum password length is 8 across all entry points
- MCP token permissions: empty permissions = no permissions, `[*]` = full permissions
- AI memory is isolated per `user_id`
- ApplyConfig automatically restores the old Caddyfile on failure
- Docker container ports bind to `127.0.0.1` by default

## Common dev commands

```bash
# Verify build
go build ./...

# Run tests (skip slower service tests)
go test $(go list ./... | grep -v internal/service) -timeout 60s

# Full test suite
go test ./... -timeout 120s

# Frontend build
cd web && npm run build
```

## User preferences

- Communicate in Chinese
- Confirm the problem before starting new implementation work
- Prefer completing large batches of related edits in one go
