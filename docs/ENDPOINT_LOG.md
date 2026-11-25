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

## GET /api/banks

- **Who may call**: a signed in user.
- **Returns**: 200 `{banks:[{code, name, supported}]}`. Four supported banks: `apex` Apex Bank, `delta` Delta Trust Bank, `savanna` Savanna Bank, `harbor` Harbor MFB. Two with `supported:false` (`cedar`, `meridian`), shown as "coming soon".
- **Frontend must**: render a card per bank; disable linking for `supported:false`. Use these `code` values everywhere (not "access" or "gtb").

## POST /api/banks/link

- **Who may call**: a signed in user who finished onboarding (BVN, PIN, name).
- **Send**: `{"bank_code": "apex", "consent": {"accepted": true, "scopes": ["accounts","balances","transactions"]}}`. All three scopes and `accepted:true` are required.
- **Returns**: 201 for a new link, 200 if the bank was already linked (idempotent, same link and account). Body: `{link_id, bank_code, bank_name, linked_at, expires_at, scopes, account:{id, link_id, bank_code, bank_name, masked_account_number, balance_kobo, unavailable, is_receiving}}`. The link expires 90 days after `linked_at`.
- **Simulated money**: a newly linked account gets the demo opening balance for that bank (Apex N70,000, Delta N60,000, Savanna N45,000, Harbor N20,000). The first linked bank becomes the receiving account.
- **Errors**: 400 `consent_required`, 400 `unknown_bank`, 400 `bank_not_supported` (coming soon banks), 403 `onboarding_required`, 401, 400 `invalid_json`.
- **Frontend must**: show a consent screen listing the three scopes with an explicit accept button before calling. Treat 200 and 201 the same. Account numbers are always masked.

## GET /api/accounts and DELETE /api/banks/{link_id}

- **Who may call**: a signed in user. Owner only: another user's link gives 404.
- **GET returns**: 200 `{accounts:[{id, link_id, bank_code, bank_name, masked_account_number, balance_kobo, unavailable, is_receiving}]}`. Use `id` as `account_id` in other calls. `is_receiving` marks the account that receives money (used by Split-Receive and by P2P).
- **DELETE returns**: 200 `{link_id, revoked: true}`. The account disappears from `GET /api/accounts`, from the dashboard and from every payment plan. If it was the receiving account, another linked account takes over (or none).
- **Errors**: 404 `not_found` (unknown or not yours), 401.
- **Frontend must**: ask "Are you sure?" before revoking. Linking the same bank again restores the account with its balance.

## GET /api/dashboard

- **Who may call**: a signed in user.
- **Query**: `limit` optional, default 20, max 100 (feed length).
- **Returns**: 200 `{total_balance_kobo, banks:[{link_id, bank_code, bank_name, balance_kobo, unavailable, expires_at, accounts:[...]}], transactions:[{id, direction, amount_kobo, bank_name, bank_code, counterparty, category, verification, posted_at}]}`.
- **Rules**: `direction` is `credit` or `debit`. `verification` is `verified` or `unverified`; a credit the bank has not confirmed is `unverified` (show a badge, it is not in the balance yet). A bank that is down has `unavailable:true` and is left out of `total_balance_kobo`. Feed is newest first and merged across all banks.
- **Seed**: Samuel sees N195,000 across four banks and an unverified N80,000 credit from "Greenfield Traders".
- **Frontend must**: show a "Bank unavailable" state on the card, and never add unavailable banks into a total yourself. Re-fetch after a payment finishes.

## GET /api/users/lookup?phone=

- **Who may call**: a signed in user.
- **Query**: `phone` in any Nigerian format (`0809...`, `+234809...`; encode `+` as `%2B`).
- **Returns**: 200 `{user:{display_name:"Bola A.", recipient_type:"apapo"}}`. Only a masked name. Never an account number, bank or full name.
- **Errors**: 400 `invalid_phone`; 400 `self_send` (your own number); 404 `not_found` for an unknown number, a user who has not finished sign up, or one with no receiving account (all look identical on purpose); 429 `rate_limited` over 30 lookups a minute.
- **Frontend must**: use this on the "send to an Apapo user" screen, show `display_name` for the sender to confirm, then send only `recipient:{apapo_phone}` to Split-Pay. Do not store or display the looked-up phone beyond what the sender typed.

## POST /api/splitpay/preview

- **Who may call**: a signed in, onboarded user.
- **Send**: `{amount_kobo, recipient, strategy?, min_balance_kobo?, max_legs?}`.
  - `recipient` is EITHER an outside account `{bank_code, account_number (10 digits), account_name?}` OR an Apapo user `{apapo_phone}`, OR `{recent_id}` from `GET /api/recipients/recent`.
  - Extra fields inside `recipient` (such as `recipient_type` and `masked_display_name`) are accepted and ignored. They never cause an error.
  - `strategy` is `fewest` (default and only option). `min_balance_kobo` is kept in every account (default 0). `max_legs` default 4.
  - `account_name` is what the user typed; if it differs from the real account name the preview flags `name_mismatch`.
- **Returns**: 200 `{legs:[{account_id, bank_code, bank_name, masked_account_number, amount_kobo, fee_kobo, state, is_collector}], total_amount_kobo, total_fee_kobo, total_debit_kobo, collector_account_id, recipient, risk:{tier, reasons[]}, flags[], duplicate_warning, shield_warning}`.
  - The planner uses the fewest accounts that cover the amount plus fees. The first leg is the collector, one of the user's own accounts. Fee is N10 (1000 kobo) per leg.
  - `recipient` for an Apapo user is only `{recipient_type:"apapo", masked_display_name}`. For outside it is `{recipient_type:"outside", bank_code, bank_name, masked_account_number, account_name}` where `account_name` is the real name on the account (use it for the "Is this who you mean?" step).
  - `risk.tier` is `low`, `unknown` or `high`. `flags` can include `reported_account`, `name_mismatch`, `first_payment`, `unusual_amount` (N100,000 or more) and `possible_duplicate`. `shield_warning` is `null` or `{credit_id, amount_kobo, sender_name, posted_at, message}`.
- **Errors**: 400 `invalid_amount`, `invalid_recipient`, `invalid_account_number`, `unknown_bank`, `bank_not_supported`, `invalid_phone`, `self_send`, `invalid_strategy`, `invalid_json`; 403 `onboarding_required`; 404 `not_found` (unknown Apapo user); 422 `insufficient_funds` (`details.spendable_kobo`, `details.shortfall_kobo`); 422 `too_many_legs` (`details.legs_needed`, `details.max_legs`); 401.
- **Seed recipients**: Apex `9000000001` is "Quick Returns Investments" (reported, tier `high`); `1000000001` at any bank is "Ada Okafor" (known good, tier `low`).
- **Frontend must**: show legs and fees before the PIN. Show the recipient name from the response. If `risk.tier` is `high` show a strong warning; payment will then need `acknowledge_risk:true` on execute. Show `shield_warning.message` when present. A bank that is down is skipped by the planner, so the plan may use other accounts.

## POST /api/splitpay/execute

- **Who may call**: a signed in, onboarded user, with the transaction PIN.
- **Headers**: `Idempotency-Key` is required (1 to 128 characters). Use a new random value (a UUID) for each payment the user confirms, and reuse the same value when retrying the same tap after a network error.
- **Send**: the same body as preview plus `pin` (4 digits) and, only for a high risk recipient, `acknowledge_risk: true`. Legs are never taken from the client: the server plans again.
- **Returns**: 202 with the payment `{id, state, total_amount_kobo, total_fee_kobo, legs[], collector_account_id, recipient, failure_reason?, reversal_attempt_count, next_reversal_retry_at, last_reversal_attempt_at, human_review, created_at, updated_at}`. Poll `GET /api/splitpay/{id}` until the state is final (see below).
- **States**: `planned`, `collecting`, `sending`, `completed`, `rolled_back`, `needs_attention`. Leg states: `pending`, `collected`, `failed`, `reversed`. Final states: `completed` and `rolled_back`. `needs_attention` is temporary: see "Hold and retry".
- **How the money moves**: each leg is pulled into the collector, one of the user's own linked accounts (never an Apapo wallet), and the collector then sends to the recipient. For an Apapo recipient the money goes to that person's receiving account, which the server resolves privately.
- **On failure**: if any leg or the final send fails, completed legs are reversed in reverse order and the payment ends `rolled_back` with the user's balances restored exactly. `failure_reason` is `insufficient_funds`, `bank_unavailable`, `timeout` or `bank_error`. A timeout is retried first with the same reference, so money cannot move twice.
- **Hold and retry**: if a reversal itself fails the payment becomes `needs_attention`. The money stays safe in the collector account. A background worker retries the reversal with the same reference (delays 1s, 2s, 4s and so on, checked every 10 seconds) until it works, then the state becomes `rolled_back`. After 10 failed attempts `human_review` becomes `true` and retrying continues. Tell the user: "Your money is safe. We are putting it back."
- **Replay**: same key and same body returns the same payment with header `Idempotent-Replay: true` and moves no money. Same key with a different body gives 409 `idempotency_conflict`.
- **Errors**: 400 `idempotency_key_required` plus every preview error; 401 `invalid_pin` (`details.attempts_left`); 423 `pin_locked` (`details.retry_after_seconds`); 409 `risk_acknowledgement_required` (`details.reasons`) when the recipient is tier `high` and `acknowledge_risk` is not true; 422 `insufficient_funds`.
- **Frontend must**: generate the key before the PIN screen, disable the button after the tap, show the final state from polling (every 1 to 2 seconds, stop on `completed` or `rolled_back`; keep going slowly on `needs_attention`), and refresh the dashboard afterwards.

## GET /api/splitpay/{id}

- **Who may call**: the user who made the payment. Anyone else gets 404 `not_found`, as if it did not exist.
- **Returns**: 200 with the same payment object execute returns: `state`, `legs[]` (with `state`), `total_amount_kobo`, `total_fee_kobo`, `recipient` (masked), `failure_reason`, `reversal_attempt_count`, `next_reversal_retry_at`, `last_reversal_attempt_at`, `human_review`.
- **Polling**: while `state` is `planned`, `collecting` or `sending`, poll every 1 to 2 seconds. `completed` and `rolled_back` are final. `needs_attention` means a refund is being retried: show "Your money is safe, we are putting it back" and keep polling every 10 seconds. `next_reversal_retry_at` is only set in that state. When `human_review` is true show "Our team is looking at this".
- **Errors**: 404 `not_found`, 401.


## GET /api/payments and GET /api/recipients/recent

- **Who may call**: a signed in user. Owner only: you only ever see your own payments and recipients.
- **GET /api/payments query**: `limit` (default 20, max 100), `before` (a payment id from `next_before`).
- **Returns**: 200 `{payments:[payment objects as in GET /api/splitpay/{id}], next_before}`. Newest first. `next_before` is the id to pass as `before` for the next page, or `null` on the last page. An unknown or foreign `before` gives 400 `invalid_cursor`.
- **GET /api/recipients/recent returns**: 200 `{recipients:[{id, recipient, last_paid_at, last_amount_kobo}]}`, up to 8, newest first, one per person, only completed payments. `recipient` is masked (an Apapo user is only `{recipient_type:"apapo", masked_display_name}`; an outside account shows bank, masked number and name).
- **Pay again**: send `recipient: {"recent_id": "<id from this list>"}` to preview or execute. The server finds the real recipient itself, so the frontend never needs a phone number or full account number for a past payee. A foreign or unknown `recent_id` gives 404.
- **Send money to another Apapo user**: look the person up with `GET /api/users/lookup?phone=`, show the masked name, then preview and execute with `recipient: {"apapo_phone": "..."}`. It is the same Split-Pay flow. The money lands in that person's receiving account; their Split-Receive rules (if any) then split it once.
- **Frontend must**: use this list for the history screen and the "recent" chips on the send screen.

## POST /api/risk/check

- **Who may call**: a signed in, onboarded user.
- **Send**: the same body as preview. `amount_kobo` may be `0` when you only want a verdict on the recipient (for example right after the user types an account number). Display fields inside `recipient` are ignored.
- **Returns**: 200 `{recipient, risk:{tier, reasons[]}, flags[], duplicate_warning}`. `tier` is `low`, `unknown` or `high`. `reasons` are plain sentences that are safe to show. `flags` can be `reported_account`, `name_mismatch`, `first_payment`, `unusual_amount`, `possible_duplicate`.
- **Rules**: `high` when at least 3 different people reported the account. A clean Apapo user, or a payee with a clean record, can be `low` even on a first payment. Anything else with a flag is `unknown`.
- **Errors**: 400 `invalid_amount`, `invalid_account_number`, `unknown_bank`, `invalid_recipient`, `invalid_phone`, `self_send`; 403 `onboarding_required`; 404 `not_found`; 401.
- **Frontend must**: colour the tier (green low, amber unknown, red high) and show `reasons` under the recipient name. For `high`, make the user confirm clearly; execute will then require `acknowledge_risk: true`.

## POST /api/reports

- **Who may call**: a signed in, onboarded user.
- **Send**: `{"recipient": {"bank_code": "harbor", "account_number": "6600000001"}, "evidence_ref": "screenshot-123"}`. `evidence_ref` is any reference to the evidence (1 to 500 characters), required.
- **Returns**: 201 `{status:"reported", message}` for a new report; 200 `{status:"already_reported"}` if this user already reported this account (one report per person per account).
- **Errors**: 400 `evidence_required`; 400 `invalid_account_number` / `unknown_bank` / `bank_not_supported`; 429 `rate_limited` after 5 new reports in 24 hours (`Retry-After` header); 403 `onboarding_required`; 401.
- **Privacy**: reports are stored only as salted hashes of the account and of the reporter. When 3 different people have reported an account its risk tier becomes `high`.
- **Frontend must**: offer "Report this account" on the Safe Send warning and after a payment. Treat 200 and 201 as success.

## GET /api/shield/check?amount_kobo=

- **Who may call**: a signed in user.
- **Query**: `amount_kobo`, a positive integer (the amount the user is about to send).
- **Returns**: 200 `{"alert": null}` when all is fine, or `{"alert": {credit_id, amount_kobo, sender_name, posted_at, message}}` when the amount is within 10 percent of a credit from the last 24 hours that the bank has NOT confirmed, and that the user has not dismissed. `message` is: "This money has not been confirmed by the bank yet. Wait for confirmation before sending it on."
- **Ignored on purpose**: Apapo's own movements and credits from other Apapo users, so a normal Apapo payment never triggers Shield.
- **Seed**: Samuel has an unconfirmed N80,000 credit from "Greenfield Traders", so any amount between N72,000 and N88,000 triggers the alert for him.
- **Errors**: 400 `invalid_amount`, 401.
- **Frontend must**: call it when the user changes the amount on the send screen (or just use `shield_warning` from preview, same data). Show a banner with the message and two buttons: "Wait" and "Send anyway". Shield warns; it never blocks.

## POST /api/shield/dismiss

- **Who may call**: the owner of the credit.
- **Send**: `{"credit_id": "crd_..."}` (from the alert).
- **Returns**: 200 `{dismissed: true}`. Repeating it is harmless. After this neither `GET /api/shield/check` nor preview's `shield_warning` shows that credit.
- **Errors**: 404 `not_found` (unknown id or someone else's credit), 400 `invalid_json`, 401.
- **Frontend must**: call it when the user taps "Send anyway" or closes the banner for good.
