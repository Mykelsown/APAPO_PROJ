# Apapo backend progress

Branch: `APAPO_BACKEND_CLEAN`. Read this first at the start of any session.

## Done

- [x] P0 setup: module, layout, README, docs, Makefile, .gitignore, .env.example, commit-msg guard hook
- [x] Section 1: `GET /api/health`, `GET /healthz`, CORS, env config, graceful shutdown, root Dockerfile, compose
- [x] Section 3 A: otp, verify-otp, bvn, pin, login, me, profile
- [x] Section 3 B: banks, link, accounts, revoke, dashboard
- [x] Section 3 C: users lookup
- [x] Section 3 D: Split-Pay preview, execute (collector, rollback, hold and retry, idempotency), status, payments, recent recipients, P2P by phone
- [x] Section 3 E: risk check, reports, shield check, shield dismiss
- [x] Section 3 F: Split-Receive rules, scan, runs and retry, reconciliation
- [x] Section 3 G: demo simulate-credit, failure, reset
- [x] Section 4: `scripts/demo.sh` (ends with "Demo complete."), frontend guide, deploy confirmation guide

## Left

- [ ] GATE: redeploy the Cube from `APAPO_BACKEND_CLEAN` and run the five checks in `docs/DEPLOY_CONFIRMATION.md`
- [ ] First real Docker image build happens on the Cube (Docker Hub was blocked where this was built)
- [ ] Frontend: turn mock mode off and match bank codes (`apex`, `delta`, `savanna`, `harbor`)

## Decisions

- Fresh code only. The branch `APAPO_BACKEND` also contains a merge of last week's Postgres rehearsal backend (commit c26c9bf). That code is NOT used here. This branch starts from commit 22410e2 (health and Docker) and never merges it.
- Go 1.24 standard library only. No third party modules. In-memory state behind one lock. Single instance.
- Money is integer kobo. Error shape `{"error":{"code","message","details"}}`.
- Tokens: HMAC signed, 24 hours, stateless. PIN: PBKDF2 with per user salt, 5 wrong tries lock for 15 minutes.
- Collector design: the legs pay into one of the user's own accounts (the largest spendable one), which then sends. Never an Apapo wallet.
- Hold and retry: a failed reversal leaves the payment `needs_attention`, money safe in the collector, the worker retries every 10 seconds with delays 1s, 2s, 4s (cap 256s), same reference, `human_review` after 10 attempts.
- A high risk recipient needs `acknowledge_risk: true` on execute (409 otherwise). Shield only warns.
- A payment to an Apapo user triggers the recipient's Split-Receive scan at once; credits use the `p2p_` reference prefix and are split once. `pay_`, `rev_`, `sr_` are never split.
- Demo seed ids are fixed (`usr_seed_*`, `acc_seed_*`) so tokens survive `POST /api/demo/reset`.
- Demo endpoints can be removed with `DEMO_ENDPOINTS=false`.
- Commits carry no tool attribution; the commit-msg hook rejects it.

## Errors met

- Docker Hub blocked in the build environment: the image could not be built there. The identical static Go build was compiled, run, health-checked and put through `scripts/demo.sh`.
- Wrong test expectations only (planner floor, hold arithmetic). No product bug was found by them.

## Next step

Redeploy the Cube and follow `docs/DEPLOY_CONFIRMATION.md`.
