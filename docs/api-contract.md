# Apapo API contract

Base URL: the Cube URL in production, `http://localhost:8080` locally. All paths start with `/api` except `/healthz`.

## Conventions

- JSON in and out, UTF-8.
- Money is always an integer number of **kobo** (`amount_kobo`, int64). N1 = 100 kobo.
- Times are ISO 8601 in UTC, for example `2026-11-25T12:00:00Z`.
- Auth: `Authorization: Bearer <access_token>` on every endpoint not marked open.
- Errors always have this shape, with a matching HTTP status:

```json
{"error":{"code":"snake_case","message":"Plain sentence.","details":{}}}
```

- CORS: origins listed in `ALLOWED_ORIGIN` may call the API from the browser. Allowed headers:
  `Authorization, Content-Type, Idempotency-Key`.

## Endpoints

| Method | Path | Auth | Status | Purpose |
|---|---|---|---|---|
| GET | `/api/health` | open | done | Connection check for the "Connected" badge |
| GET | `/healthz` | open | done | Plain text `ok` for container health checks |
| POST | `/api/auth/otp` | open | done | Ask for a sign in code (always 200 for a valid phone) |
| POST | `/api/auth/verify-otp` | open | done | Check the code, get `access_token` |
| POST | `/api/auth/bvn` | token | done | Mock BVN check (any 11 digits) |
| POST | `/api/auth/pin` | token | done | Set the 4 digit PIN once, after BVN |
| POST | `/api/auth/login` | open | done | Phone and PIN sign in for finished users |
| GET | `/api/me` | token | done | Profile, onboarding state, receiving account |
| PUT | `/api/me/profile` | token | done | Change display name |
| GET | `/api/banks` | token | done | Bank catalogue, with two "coming soon" entries |
| POST | `/api/banks/link` | token | done | Link a bank after consent (idempotent) |
| GET | `/api/accounts` | token | done | The user's linked accounts, masked, with balances |
| DELETE | `/api/banks/{link_id}` | token | done | Revoke a bank link |
| GET | `/api/dashboard?limit=` | token | done | Total balance, a card per bank, merged transaction feed |
| GET | `/api/users/lookup?phone=` | token | done | Find another Apapo user by phone, masked name only |
| POST | `/api/splitpay/preview` | token | done | Plan a payment (legs, fees, risk, warnings). Moves no money |
| POST | `/api/splitpay/execute` | token + PIN | done | Run a payment. Needs `Idempotency-Key`. Answers 202 |
| GET | `/api/splitpay/{id}` | token | done | Payment status, legs, reversal retry info. Owner only |
| GET | `/api/payments?limit=&before=` | token | done | The user's payment history, newest first, id cursor |
| GET | `/api/recipients/recent` | token | done | People the user paid before, masked |
| POST | `/api/risk/check` | token | done | Safe Send verdict for a recipient (same body as preview) |
| POST | `/api/reports` | token | done | Report a suspicious outside account (evidence required) |
| GET | `/api/shield/check?amount_kobo=` | token | done | Warn if the amount is near an unconfirmed credit |
| POST | `/api/shield/dismiss` | token | done | Hide the Shield alert for one credit |
| PUT | `/api/splitreceive/rules` | token + PIN | done | Save the user's Split-Receive rules (needs standing consent) |
| GET | `/api/splitreceive/rules` | token | done | Read the rules (`{"rules": null}` if none) |
| POST | `/api/splitreceive/scan` | token | done | Split new confirmed credits now (also runs every 10 seconds) |

## Common error codes

| HTTP | code | When |
|---|---|---|
| 404 | `not_found` | Unknown path or a resource you cannot see |
| 405 | `method_not_allowed` | Path exists, wrong method |
| 403 | `origin_not_allowed` | CORS preflight from an origin not in `ALLOWED_ORIGIN` |
| 500 | `internal_error` | Unexpected server fault |
