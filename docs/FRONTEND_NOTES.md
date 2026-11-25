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

- Flow for a new person: `POST /api/auth/otp`, `POST /api/auth/verify-otp`, then `POST /api/auth/bvn`, `POST /api/auth/pin`.
- Flow for the two demo users: `POST /api/auth/login` with phone and PIN, no code needed.
- The code is `123456` unless the backend sets `DEV_OTP`. Show a "demo code: 123456" hint in the UI.
- Show the rate limit as a friendly message using `Retry-After` (seconds).

## After sign in

- Call `GET /api/me` and branch: `bvn_verified` false means the BVN screen; `pin_set` false means the PIN screen; `receiving_account` null means the user should link a bank first.
- `display_name` is what other Apapo users see only as a masked name like "Bola A." (see people lookup).

## Send money (preview)

- Amounts are kobo. N150,000 is `15000000`.
- Send the recipient exactly as you have it; extra display fields are fine and ignored.
- Call preview whenever the amount or recipient changes, then once more right before the PIN screen. Execute plans again on the server, so never send legs yourself.

## Sending to another Apapo user (P2P)

- Lookup, preview, execute with `recipient: {apapo_phone}`. Show only `masked_display_name`.
- After it completes the recipient's dashboard shows a verified credit from "Samuel P." (the sender, masked).
- Recent recipients come with an `id`; pass it as `recipient.recent_id` to pay them again without keeping phone numbers.

## Safe Send and Shield

- `risk.tier` colours: `low` green, `unknown` amber, `high` red. Show `risk.reasons` as plain text.
- A `high` recipient needs a clear extra confirmation. Then send `acknowledge_risk: true` with execute; without it execute answers 409 `risk_acknowledgement_required` and moves no money.
- Shield never blocks. Show its message and let the user decide.
- Demo path: preview N80,000 for Samuel for the Shield banner; Apex `9000000001` for the red warning.

## Split-Receive screens

- Rules screen: `GET` then `PUT /api/splitreceive/rules` (consent checkbox and PIN).
- Activity screen: `GET /api/splitreceive/runs` plus a "Check for new money" button (`POST /api/splitreceive/scan`).
- Statement screen: `GET /api/reconciliation`.
- To demo a salary arriving, call `POST /api/demo/simulate-credit` (next section), then refresh.
