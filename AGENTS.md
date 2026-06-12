# AGENTS.md

## Cursor Cloud specific instructions

ZENLENET PingMesh is a single self-contained Go binary (pure Go, `CGO_ENABLED=0`) — a network-quality monitoring platform with an embedded SQLite DB (`modernc.org/sqlite`), embedded frontend assets, and an HTTP UI. There is **no external service** (no DB server, etc.) to provision; everything is in one process. See `readme.md` for full feature/usage docs.

### Build / lint / test / run
- Build (dev): `CGO_ENABLED=0 go build -ldflags="-s -w" -o pingmesh ./src` (the `main` package lives in `./src`, not the repo root).
- Lint: `go vet ./src/...`.
- Test: `go test ./src/...` — note the repo currently has **no test files**, so this is a no-op pass.
- Run: `./pingmesh` (listens on `:8899`). Optional flags in `readme.md` (`-p`, `-l`, `-w`, `-join`, ...).

### Non-obvious gotchas
- **ICMP needs raw-socket capability.** After every rebuild, run `sudo setcap cap_net_raw+ep ./pingmesh` (binary is rebuilt fresh and loses the cap). Without it the UI still loads but all pings report 100% loss. Running as root also works.
- **Go version**: `go.mod` requires Go **1.25.0** (already installed). The `Dockerfile` pins `golang:1.22-alpine` and `readme.md` says "1.22+", which is stale and would fail to build — use the host Go toolchain for dev, not the Dockerfile.
- **First start auto-initializes** `conf/`, `db/`, `logs/`, `html/` from embedded assets and creates a default admin user.
- **Default login**: `admin` / `admin123`. Login API is `POST /api/login.json` (form fields `username`, `password`); it sets a session cookie. APIs live under `/api/*.json`.
- Runtime artifacts (`db/database.db`, `conf/config.json`, `logs/*`, the `pingmesh` binary) are gitignored — do not commit them.
