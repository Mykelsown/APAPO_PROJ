# Notes for the frontend

Short, plain notes per endpoint. The full table is in `docs/api-contract.md`.

## General

- Money is integer kobo. Divide by 100 only when displaying. Never send naira decimals.
- Every error has `error.code` (stable, switch on this) and `error.message` (safe to show).
- Local dev: either set `ALLOWED_ORIGIN=http://localhost:5173` on the backend, or proxy `/api` from the dev server to `http://localhost:8080`.

## Health (`GET /api/health`)

- Call it on startup for the "Connected" badge. Success means HTTP 200 and `status === "ok"`.
- A fetch error or timeout (try 5 seconds) means "Not connected". There is no JSON in that case.
- No token needed. Do not call `/healthz` from the app; it is for the container.
- If the browser shows a CORS error, the backend's `ALLOWED_ORIGIN` does not include your exact origin (scheme, host and port, no trailing path).

## Sign in

- Flow for a new person: `POST /api/auth/otp`, `POST /api/auth/verify-otp`, then (next endpoints) `POST /api/auth/bvn`, `POST /api/auth/pin`.
- Flow for the two demo users: `POST /api/auth/login` with phone and PIN, no code needed.
- The code is `123456` unless the backend sets `DEV_OTP`. Show a "demo code: 123456" hint in the UI.
- Show the rate limit as a friendly message using `Retry-After` (seconds).
