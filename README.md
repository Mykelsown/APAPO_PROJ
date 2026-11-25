# Apapo backend

Apapo links a person's accounts across several banks and lets them treat them as one:

- **Unified dashboard**: one total balance and one merged transaction feed.
- **Split-Pay**: pay one amount from several accounts at once, with automatic rollback if any leg fails.
- **Split-Receive**: divide incoming money across accounts by rule, with a settlement ledger.
- **Safe Send**: risk tiers for recipients, based on community reports.
- **Shield**: warns before you send on money the bank has not confirmed yet.
- **Send to an Apapo user** by phone number.

Built for the Ogun Innovate Hackathon (Fintech track).

## Banks are simulated

There is no real bank integration. Every bank in this project is **simulated** by a mock provider
inside the server, with fictional bank names. No real bank's name, products or branding are used.
The server refuses to start with `PROVIDER=real`. All data lives in memory, so a restart resets it.

## Run locally

Requires Go 1.24 (standard library only, no third-party modules).

```bash
make run          # starts on http://localhost:8080
make check        # gofmt, go vet, go test -race
make build        # static binary in backend/bin/apapo-api
```

With Docker:

```bash
make docker                       # builds image apapo-api:dev from the root Dockerfile
docker run --rm -p 8080:8080 -e APP_SECRET=change-me apapo-api:dev
# or
cp backend/.env.example .env      # edit values, then
make up                           # docker compose up --build
```

Check it: `curl http://localhost:8080/api/health` and `curl http://localhost:8080/healthz`.

Install the commit message guard once per clone: `make hooks`.

## Configuration

All settings come from environment variables. See `backend/.env.example`.

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | Listen port, on `0.0.0.0` |
| `ADDR` | empty | Full listen address, overrides `PORT` |
| `APP_SECRET` | random per start | Signs access tokens. Set it in any shared deploy, or tokens die on restart |
| `ALLOWED_ORIGIN` | empty | Comma separated CORS origins. `*` echoes any origin (demo only) |
| `DEV_OTP` | `123456` | Fixed sign in code. Demo only |
| `PROVIDER` | `mock` | Only `mock` exists |
| `TRUST_PROXY` | `false` | Trust `X-Forwarded-For` for client IP |
| `VERSION` | build value or `dev` | Reported by `/api/health` |
| `DEMO_ENDPOINTS` | `true` | `false` hides `/api/demo/*` (mock helpers) |

## Deploy on Nobus (Cube)

Nobus builds the image straight from this repository using the `Dockerfile` at the repository root.

1. Open the Nobus console and choose **Create Cube**.
2. **Group Name**: for example `apapo`.
3. **Deployment Type**: `Standard`.
4. **Service Configuration**: `Custom Repository`.
   - **Repository URL**: `https://github.com/Mykelsown/APAPO_PROJ`
   - **Branch**: `APAPO_BACKEND` (or whichever branch holds the finished code)
   - **Container Port**: `8080`
   - **Git username / token**: only if the repository is private. Type the token into the form only; never write it in any file.
   - Tick **Dockerfile Present**. Leave **Docker Compose Present** unticked.
5. **CPU / Memory**: small is enough, for example `0.5` CPU and `512m`.
6. **Mounts / extra disk**: none. State is in memory.
7. **Environment variables**:
   - `APP_SECRET` = a long random string (`openssl rand -hex 32`)
   - `DEV_OTP` = `123456`
   - `PROVIDER` = `mock`
   - `ALLOWED_ORIGIN` = the frontend URL, for example `https://apapo-frontend.example`, or `*` while testing
   - `TRUST_PROXY` = `true` (Cube sits behind a proxy)
   - `VERSION` = for example `0.1.0`
8. **Server name**: for example `apapo-api`. Pick the smallest **spec**. Click **Launch**.

**Confirm**: open `https://<cube-url>/api/health`. You should see JSON with `"status":"ok"` and
`"message":"Backend is reachable"`. `https://<cube-url>/healthz` should show `ok`.

**Redeploy after a new commit**: push the commit to the branch, then use the Cube's redeploy or rebuild
action in the console (ask Nobus whether it also rebuilds on its own). Data resets on every redeploy.

**Logs**: open the Cube in the console and view its logs. On start the server prints one line with the
settings (secrets masked), then one line per request (method, path, status, duration).

### Questions for Nobus staff

- What public URL or domain does a Cube get, and can we choose it?
- Is it HTTPS by default? Is a certificate provided?
- Does a Cube rebuild automatically on a new commit, or only when we click redeploy?
- Is the Dockerfile expected at the repository root (it is, here)?
- Does the platform's proxy send `X-Forwarded-For`, and does it time out long requests?
- Where are container logs shown, and for how long are they kept?
- Does the Cube honour the image `HEALTHCHECK`, or does it run its own probe?

## Project layout

```
Dockerfile, docker-compose.yml, .dockerignore   build and run (repo root, for Nobus)
backend/cmd/api                                 main program
backend/internal/...                            config, HTTP helpers, server and features
docs/api-contract.md                            endpoint table for the frontend
docs/ENDPOINT_LOG.md                            one report per finished endpoint
docs/FRONTEND_NOTES.md                          plain notes for the frontend dev
scripts/                                        demo script and git hook
```
