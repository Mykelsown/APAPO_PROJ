# Apapo backend progress

Branch: `APAPO_BACKEND`. Read this first at the start of any session and continue from the first unchecked item.

## Done

- [x] P0 setup: module, layout, README, docs, Makefile, .gitignore, .env.example, commit-msg guard hook

## Left

- [ ] Section 1: `GET /api/health`, `GET /healthz`, CORS, config, graceful shutdown, Docker (gate, then stop)
- [ ] Section 3 A: sign in and profile
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
- Commit-msg hook lives in `.git/hooks/commit-msg`; a copy is kept in `scripts/commit-msg-hook.sh` (`make hooks` installs it).

## Errors met

- Docker daemon was not running in the build machine; started `dockerd` manually. Not a project issue.

## Next step

Build the health endpoints, CORS, config and Docker files, test the image, commit, then stop for the Nobus deploy.
