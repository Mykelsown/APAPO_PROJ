# Apapo backend progress

Branch: `APAPO_BACKEND`. Read this first at the start of any session and continue from the first unchecked item.

## Done

- [x] P0 setup: module, layout, README, docs, Makefile, .gitignore, .env.example, commit-msg guard hook
- [x] Section 1: `GET /api/health`, `GET /healthz`, CORS, env config, graceful shutdown, root Dockerfile, .dockerignore, compose. Image built, run, both endpoints curled, HEALTHCHECK reports healthy, `docker stop` drains cleanly, `PROVIDER=real` refused.

## Left

- [ ] GATE: user deploys to Nobus Cube and confirms the frontend reaches `/api/health`. Wait for "continue".
- [ ] Section 3 A: sign in and profile (done: otp, verify-otp)
- [ ] Section 3 B: banks and dashboard
- [ ] Section 3 C: people lookup
- [ ] Section 3 D: Split-Pay
- [ ] Section 3 E: Safe Send and Shield
- [ ] Section 3 F: Split-Receive and reconciliation
- [ ] Section 3 G: demo helpers
- [ ] Section 4: demo script, Docker end-to-end run, final docs

## Decisions

- Local git identity set to Mykelsown for this repo only (it was the tool's default identity). Global config untouched.
- Go 1.24 standard library only. No third-party modules.
- CORS: `ALLOWED_ORIGIN=*` echoes any origin (demo convenience); a literal `*` is never sent. No `Allow-Credentials`, since auth is a bearer header, not cookies. Blocked preflight answers 403 `origin_not_allowed`; blocked normal requests are served without CORS headers so the browser blocks them.
- `APP_SECRET` unset: a random secret is generated per start with a warning (tokens reset on restart).
- Docker HEALTHCHECK runs the binary itself with `-healthcheck`, so the runtime image needs no curl or wget.
- Runtime image installs no packages (CA bundle copied from the build stage), so the build needs network only to pull base images.
- Unknown paths and wrong methods return the standard JSON error (404 `not_found`, 405 `method_not_allowed`).
- Commit-msg hook lives in `.git/hooks/commit-msg`; a copy is kept in `scripts/commit-msg-hook.sh` (`make hooks` installs it).

## Errors met

- Docker daemon was not running in the build machine; started `dockerd` manually. Not a project issue.
- `POST /api/health` returned 404 instead of 405. Cause: a catch-all `/api/` route matched every method. Fix: removed it and converted the mux's own 404 and 405 into JSON.
- `apk add` failed during `docker build` with a DNS error. Cause: RUN steps in this sandbox have no network. Fix: dropped apk and copied CA certificates from the build stage.

## Next step

Wait for "continue" after the Nobus deploy. Then start section 3 A: `POST /api/auth/otp` and `POST /api/auth/verify-otp` (store, tokens, rate limits, seeded users).
