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

## POST /api/auth/bvn

- **Who may call**: a signed in user (even one still onboarding).
- **Send**: `{"bvn": "22212345678"}`, exactly 11 digits. Mock check: any 11 digits pass.
- **Returns**: 200 `{bvn_verified: true, display_name, onboarding}`. If the user had no name yet, the mock identity service fills `display_name` with a name.
- **Never returned**: the BVN itself. The server keeps only a hash and the last 4 digits.
- **Errors**: 400 `invalid_bvn`, 400 `invalid_json`, 401 `unauthorized` / `invalid_token`.
- **Frontend must**: ask for the BVN once after OTP when `onboarding.bvn_verified` is false, then go to PIN. Pre-fill the returned `display_name` where a name is shown.

## POST /api/auth/pin

- **Who may call**: a signed in user whose BVN is verified.
- **Send**: `{"pin": "4821"}`, exactly 4 digits.
- **Returns**: 200 `{pin_set: true, onboarding}`. After this `onboarding.complete` is true (BVN, PIN and a display name are all there).
- **Errors**: 400 `invalid_pin_format`; 400 `weak_pin` (0000, 1234, 4321 and repeated digits); 403 `bvn_required`; 409 `pin_already_set`; 401 as usual.
- **Storage**: PBKDF2 hash with a per user salt. The PIN is never returned or logged.
- **Frontend must**: ask for the PIN twice on the client (confirm field) and send it once. Wrong PIN rules for payments are on the Split-Pay endpoints.

## POST /api/auth/login

- **Who may call**: anyone. No token.
- **Send**: `{"phone": "08012345678", "pin": "4821"}`.
- **Returns**: 200 `{access_token, expires_at, user, onboarding}` (same shape as verify-otp).
- **Errors**: 401 `invalid_credentials` (wrong PIN, unknown phone, or a user who has not set a PIN yet; all look the same on purpose, wrong PIN adds `details.attempts_left`); 423 `pin_locked` after 5 wrong PINs for 15 minutes (`details.retry_after_seconds` and `Retry-After`); 429 `rate_limited` (per IP).
- **Demo users**: Samuel `08012345678` PIN `4821`, Bola `08098765432` PIN `5912`.
- **Frontend must**: use this for returning users instead of the OTP flow. On 423 show a countdown from `retry_after_seconds`.

## GET /api/me and PUT /api/me/profile

- **Who may call**: a signed in user.
- **GET returns**: 200 `{user:{id, phone, display_name}, onboarding:{bvn_verified, pin_set, complete}, receiving_account}`. `receiving_account` is `null` until the user has linked a bank (then `{account_id, bank_code, bank_name, masked_account_number}`). Never the full account number.
- **PUT sends**: `{"display_name": "Bola Adeyemi"}`, 1 to 80 characters; extra spaces are trimmed. Returns the same body as GET.
- **Errors**: 400 `invalid_display_name`, 400 `invalid_json`, 401.
- **Frontend must**: call GET once after sign in and use `onboarding` to decide which screen comes next (BVN, PIN, link banks, dashboard). Do not poll it.
