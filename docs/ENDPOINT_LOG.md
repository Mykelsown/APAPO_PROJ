# Endpoint log

One plain-words report per finished endpoint, newest last. Forward to the frontend dev.

## GET /api/health and GET /healthz

- **Who may call**: anyone, no sign in.
- **Send**: nothing.
- **`GET /api/health` returns** 200 with JSON: `status` ("ok"), `service` ("apapo-api"), `version`, `provider` ("mock"), `time` (ISO UTC), `message` ("Backend is reachable").
- **`GET /healthz` returns** 200 with the plain text `ok`. Meant for Docker and uptime checks, not the app.
- **Errors**: a wrong method gets 405 `method_not_allowed`; an unknown path gets 404 `not_found`; a browser preflight from an origin not in `ALLOWED_ORIGIN` gets 403 `origin_not_allowed`. If the server is down, the request fails at the network level (no JSON at all).
- **Frontend must**: call `/api/health` on app load and show "Connected" only when the call succeeds and `status` is "ok". Treat a network error or timeout as "Not connected". Do not show version or time to users unless useful for debugging. Do not use `/healthz` from the app.

## POST /api/auth/otp and POST /api/auth/verify-otp

- **Who may call**: anyone. No token.
- **otp sends**: `{"phone": "08012345678"}`. `+2348012345678` and `2348012345678` also work.
- **otp returns**: 200 `{"sent": true, "expires_in_seconds": 300, "message": "..."}`. It answers 200 for known and unknown numbers alike, so it never reveals who has an account.
- **verify-otp sends**: `{"phone": "...", "code": "123456"}`. In this demo every user's code is the server's `DEV_OTP` (default `123456`).
- **verify-otp returns**: 200 `{access_token, expires_at, user:{id, phone, display_name}, onboarding:{bvn_verified, pin_set, complete}}`. A new number creates a new user with `onboarding.complete:false`.
- **Errors**: 400 `invalid_phone`; 400 `invalid_json`; 401 `invalid_code` (`details.attempts_left`); 423 `otp_locked` after 5 wrong codes (`details.retry_after_seconds`, `Retry-After` header, 10 minutes); 429 `rate_limited` on the 4th code request for one phone inside 10 minutes, or more than 30 calls a minute from one IP on any `/api/auth/*` route (`Retry-After` header).
- **Frontend must**: send the phone as typed (the server normalises it). Store `access_token` and send it as `Authorization: Bearer <token>` on every other call. Token lasts 24 hours; on any 401 `invalid_token` go back to sign in. If `onboarding.complete` is false, continue to BVN then PIN.
