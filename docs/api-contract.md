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

## Common error codes

| HTTP | code | When |
|---|---|---|
| 404 | `not_found` | Unknown path or a resource you cannot see |
| 405 | `method_not_allowed` | Path exists, wrong method |
| 403 | `origin_not_allowed` | CORS preflight from an origin not in `ALLOWED_ORIGIN` |
| 500 | `internal_error` | Unexpected server fault |
