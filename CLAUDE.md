# Jellysweep — AI Assistant Guide

## Project Overview

Jellysweep (`github.com/jon4hz/jellysweep`) is a smart cleanup tool for Jellyfin media servers. It scans libraries via Sonarr/Radarr, applies configurable filters to identify unused or old media, marks items for deletion using tags, and removes them after a grace period. Users can request to keep content via a web UI; admins review requests and make final decisions.

**Tech stack:** Go 1.25, Gin (HTTP), GORM + SQLite, Viper (config), Cobra (CLI), charmbracelet/log, a-h/templ (HTML), Tailwind CSS 4, esbuild, Chart.js.

---

## Repository Structure

```
jellysweep/
├── main.go                        # Entry point — calls cmd.Execute()
├── cmd/                           # Cobra CLI commands
│   ├── root.go                    # Root command + persistent flags
│   ├── serve.go                   # "serve" — starts engine + API server
│   ├── healthcheck.go             # "healthcheck" command
│   └── generate-vapid-keys.go    # "generate-vapid-keys" command
├── internal/                      # Private packages
│   ├── api/
│   │   ├── auth/                  # Auth providers (OIDC, Jellyfin, API key)
│   │   ├── handler/               # Gin HTTP handlers
│   │   └── models/                # API request/response types + converters
│   ├── cache/                     # Image cache + engine data cache
│   ├── config/                    # Viper-based config loading & validation
│   ├── database/                  # GORM/SQLite models, queries, interface
│   ├── engine/                    # Core business logic
│   │   ├── arr/                   # Sonarr/Radarr integration
│   │   │   ├── sonarr/
│   │   │   └── radarr/
│   │   ├── jellyfin/              # Jellyfin API client
│   │   └── stats/                 # Viewing stats providers
│   │       ├── jellystat/
│   │       └── streamystats/
│   ├── filter/                    # Filterer interface + per-library filters
│   │   ├── age_filter/
│   │   ├── database_filter/
│   │   ├── series_filter/
│   │   ├── size_filter/
│   │   ├── stream_filter/
│   │   ├── tags_filter/
│   │   └── tunarr_filter/
│   ├── gravatar/                  # Gravatar profile picture support
│   ├── logging/                   # Log level setup
│   ├── notify/
│   │   ├── email/                 # SMTP email notifications
│   │   └── webpush/               # Web push (VAPID) notifications
│   ├── policy/                    # Deletion policy engine
│   │   ├── default.go
│   │   └── disk_usage.go
│   ├── scheduler/                 # gocron-based task scheduler
│   ├── static/                    # Embedded static assets (fs.FS)
│   ├── tags/                      # Sonarr/Radarr tag management
│   └── version/                   # Build version string
├── pkg/                           # Reusable external-service clients
│   ├── jellyseerr/                # Jellyseerr API client
│   ├── jellystat/                 # Jellystat API client
│   ├── streamystats/              # Streamystats API client
│   └── tunarr/                    # Tunarr API client
├── web/templates/                 # a-h/templ templates
│   ├── layout.templ               # Base HTML layout
│   ├── components/                # Reusable UI components
│   └── pages/                     # Full-page templates
├── src/
│   └── chart.js                   # JS entry point bundled by esbuild
├── input.css                      # Tailwind CSS entry point
└── assets/                        # Static assets (screenshots, shell scripts)
```

---

## Build & Development Commands

### Primary workflow

```bash
# Install Node dependencies (first time)
npm install --include=dev

# Build all assets (templ → Go, CSS, JS) — required before running
make build

# Run with debug logging (also calls make build first)
make run

# Run tests
go test ./...

# Run a specific test
go test -run TestName ./path/to/package

# Lint
golangci-lint run
```

### Individual build steps

```bash
make templ    # go tool templ generate -v    (regenerates *_templ.go files)
make css      # npm run build-css            (Tailwind → dist/style.css)
make js       # npm run build-js             (esbuild → dist/chart.js)
make clean    # Remove generated *_templ.go and dist/
```

### Hot-reload development

```bash
# Requires: go install github.com/air-verse/air@latest
air           # Watches .go/.templ/.html files, rebuilds & restarts automatically
```

The `.air.toml` config builds CSS, generates templ, compiles Go, and runs `serve --log-level=debug`.

### Docker (dev)

```bash
docker compose -f compose.dev.yml up
```

---

## Key Architectural Patterns

### Startup sequence (`cmd/serve.go`)

1. `config.Load()` — reads YAML + env vars via Viper
2. `database.New()` — opens SQLite, runs GORM auto-migrations
3. `engine.New()` — wires up all service clients, cache, scheduler, policy engine
4. `api.New()` — sets up Gin router with auth middleware and handlers
5. `engine.Run(ctx)` in a goroutine — runs the scheduler
6. `server.Run(ctx)` in a goroutine — starts HTTP listener

### Engine (`internal/engine/`)

The engine is the central coordinator. It owns:
- Service clients (Sonarr, Radarr, Jellyfin, Jellystat/Streamystats, Jellyseerr, Tunarr)
- The policy engine (`internal/policy/`)
- The filter chain (`internal/filter/`)
- The database client
- Notification clients (email, webpush, ntfy)
- Cache instances

The main scan-and-mark cycle runs on a cron schedule. The cleanup phase (`cleanupMedia`) checks policy then deletes via Sonarr/Radarr and Jellyfin.

### Filter system (`internal/filter/`)

All filters implement `Filterer`:
```go
type Filterer interface {
    fmt.Stringer
    Apply(context.Context, []arr.MediaItem) ([]arr.MediaItem, error)
}
```

`filter.Filter.ApplyAll()` applies filters sequentially; any filter can exclude an item from consideration. Filters run **before** items are tagged/tracked. Once an item is in the database (marked for deletion), filters no longer apply to it.

### Policy engine (`internal/policy/`)

Policies implement `Policy` and determine **when** a tracked item should actually be deleted (respecting grace periods and disk usage thresholds). Policies run **after** items are in the database.

### Database interface (`internal/database/interface.go`)

The `DB` interface composes four sub-interfaces: `UserDB`, `MediaDB`, `RequestDB`, `HistoryDB`. The concrete implementation is `database.Client` (GORM + SQLite). Tests should mock the `DB` interface.

### Authentication (`internal/api/auth/`)

Uses a factory (`factory.go`) that registers providers. Supported: OIDC (tested with Authentik), Jellyfin (all Jellyfin admins become Jellysweep admins), and API key. At least one auth method must be enabled.

### Templates (`web/templates/`)

Templates are written in `.templ` files and compiled to `*_templ.go` by `go tool templ generate`. **Never edit `*_templ.go` files directly** — they are generated. Always edit the corresponding `.templ` file and regenerate.

### Configuration (`internal/config/`)

Uses Viper with `JELLYSWEEP_` env prefix. Library-specific config (filters, thresholds) **cannot** be set via env vars — it requires a YAML config file. Library name lookup is case-insensitive (handled by `GetLibraryConfig()`). Struct tags must have both `yaml:` and `mapstructure:` annotations.

---

## Code Style & Conventions

### Formatting

- **Formatter:** `gofumpt` + `goimports` (enforced by golangci-lint and pre-commit)
- **Import order:** stdlib → external packages → internal (`github.com/jon4hz/jellysweep/...`)

### Go conventions

- Use `context.Context` as the first parameter for any function that performs I/O
- Return errors; use `//nolint:errcheck` only when intentionally discarding
- Config structs must have both `yaml:"..."` and `mapstructure:"..."` struct tags
- Prefer table-driven tests: `tests := []struct{ ... }{ ... }`
- Use `require` for fatal test setup steps, `assert` for non-fatal assertions
- Mock external dependencies in tests using interface mocks (see existing `*_test.go` files)
- Exported types and functions must have a doc comment

### Logging

Use `github.com/charmbracelet/log`:
```go
log.Info("message", "key", value)
log.Error("message", "key", value, "error", err)
log.Debug("message", "key", value)
```

### Error handling pattern

```go
if err := someOp(ctx); err != nil {
    log.Error("failed to do thing", "key", val, "error", err)
    return fmt.Errorf("context: %w", err)
}
```

---

## Linting

golangci-lint v2 is configured in `.golangci.yaml`. Enabled linters include: `bodyclose`, `exhaustive`, `goconst`, `gosec`, `misspell`, `nilerr`, `noctx`, `prealloc`, `unconvert`, `unparam`, `whitespace`, and more. Formatters: `gofumpt`, `goimports`.

Run lint locally:
```bash
golangci-lint run
```

---

## Pre-commit Hooks

Install once:
```bash
pip install pre-commit
pre-commit install
```

Hooks run on commit:
- `trailing-whitespace`, `end-of-file-fixer`, `check-yaml`, `check-json`, `check-merge-conflict`
- `gitleaks` — secret detection
- `go-fmt`, `go-mod-tidy`
- `golangci-lint`
- `mdformat` (with `mdformat-gfm`)
- `make build` — rebuilds all generated assets when `.go`, `.templ`, `.js`, or `.css` files change

---

## CI (GitHub Actions)

Two jobs defined in `.github/workflows/ci.yml`:

| Job | What it runs |
|-----|-------------|
| `lint` | `golangci-lint-action` |
| `test` | `go test -v -coverprofile=coverage.out ./...` |

CI skips on changes to `docs/**`, `**.md`, and `zensical.toml`.

---

## Adding a New Filter

1. Create `internal/filter/<name>_filter/filter.go`
2. Implement `filter.Filterer` interface
3. Wire the filter into the engine's filter chain (in `internal/engine/`)

## Adding a New Notification Provider

1. Create `internal/notify/<name>/` with a client struct
2. Add config struct to `internal/config/config.go` (with `yaml:` + `mapstructure:` tags)
3. Add defaults + env binding in `config.go`
4. Wire into `engine.New()` in `internal/engine/`

## Adding a New API Endpoint

1. Add handler method on `*handler.Handler` in `internal/api/handler/`
2. Register the route in `internal/api/` (the Gin router setup)
3. Use `getUser(c)` to extract the authenticated user; return early if nil

## Modifying Templates

1. Edit the `.templ` source file
2. Run `make templ` to regenerate `*_templ.go`
3. The generated files are committed — include them in your PR

---

## Configuration Reference

The app uses YAML config + `JELLYSWEEP_*` env vars (env vars override file values). Library config requires the YAML file.

Key required fields:
- `session_key` — random string (`openssl rand -base64 32`)
- `jellyfin.url` + `jellyfin.api_key`
- Either `sonarr` or `radarr` (or both)
- Either `jellystat` or `streamystats` (not both)
- At least one auth method (`auth.oidc` or `auth.jellyfin`)
- At least one `libraries` entry matching a Jellyfin library name exactly

Default port: `0.0.0.0:3002`. Default cleanup schedule: every 12 hours (`0 */12 * * *`). `dry_run` defaults to `true` — set to `false` for actual deletions.
