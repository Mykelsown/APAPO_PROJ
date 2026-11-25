# Frontend guide: what to build and in what order

Written for the frontend developer. It describes screens and the calls behind them in plain words.
Exact request and response fields are in `docs/api-contract.md` (table) and `docs/ENDPOINT_LOG.md`
(one report per endpoint). Small gotchas are in `docs/FRONTEND_NOTES.md`.

## 0. Set up the connection first

1. Point the app at the backend base URL (local: `http://localhost:8080`, deployed: the Nobus Cube URL).
2. Turn mock mode off. Many "it does not connect" problems are the app still running on its mock data.
3. On start, call `GET /api/health`. Show a small "Connected" badge when it answers `status: ok`, and "Not connected" on any failure or after 5 seconds.
4. Tell the backend person your exact origin so they put it in `ALLOWED_ORIGIN` (CORS).

## 1. Rules that apply everywhere

- Money is whole kobo (N1 = 100). Convert only when drawing the screen.
- Every error has a stable `error.code`. Branch on the code, show `error.message`.
- Send the token as a bearer header on every call except sign in, health and the demo tools.
- A 401 `invalid_token` anywhere means: clear the token and return to sign in.
- Bank codes are `apex`, `delta`, `savanna`, `harbor` (not "access" or "gtb"). Use `GET /api/banks` as the source.
- Never show or store full account numbers. The API only gives masked ones.

## 2. Screens, in the order to build them

**Sign in and sign up.** Returning users: phone and PIN. New users: phone, code, BVN, PIN. The demo code is shown as a hint. After any sign in, read the profile and decide where to go next from its onboarding flags.

**Link banks.** List the catalogue, show a consent screen (accounts, balances, transactions) with an explicit accept, then link. "Coming soon" banks are visible but disabled. Revoking is a confirm dialog.

**Dashboard.** One total, a card per bank (a down bank shows "unavailable" and is not in the total), and one merged feed. Credits the bank has not confirmed carry an "unverified" badge.

**Send money.** One flow for outside accounts and for Apapo users:
1. Choose the recipient: type bank and account number, or look up a phone number, or tap a recent recipient.
2. Enter the amount. Call preview. Show the legs, the N10 fee per leg, the recipient name, the risk colour and reasons, and the Shield banner if present.
3. If the risk is high, require a deliberate extra confirmation, and remember to say so when executing.
4. Ask for the PIN. Make a fresh idempotency key before this screen and keep it for retries of the same tap.
5. Execute, then watch the payment until it ends. Show "Your money is safe, we are putting it back" if it needs attention.
6. On completion refresh the dashboard and history.

**History and recent recipients.** A paged list of payments and a row of masked recent payees.

**Split-Receive.** A rules screen (consent, PIN, up to five destinations, percent or fixed amount, remainder stays), an activity screen with a "check for new money" button and retry for held shares, and a statement screen showing sent, held and kept per credit, with a balanced tick.

**Safe Send report.** A "report this account" action with an evidence reference.

**Hidden demo panel.** Buttons for: simulate a salary, simulate an unconfirmed credit, switch a failure on or off, reset. Keep it out of the main navigation.

## 3. Demo data you will see

- Samuel Pelumi, phone 08012345678, PIN 4821: four banks, N195,000 in total, one unconfirmed N80,000 credit.
- Bola Adeyemi, phone 08098765432, PIN 5912: one Apex account.
- Reported account: Apex 9000000001 (red warning). Known good payee: any bank, 1000000001 (green).

## 4. How to rehearse the whole story

Run `scripts/demo.sh <base url>` once to watch the backend do the full story, then recreate the same
steps in the app: sign in both users, set Bola's rules, send Samuel's N20,000 to Bola, simulate a salary,
trigger the red warning, force a failure, force the reversal outage, finish with a clean N150,000 payment.
Always press reset before presenting.

## 5. Things that are not bugs

- Data lives in memory. A redeploy or restart wipes it, and tokens die unless the backend has a fixed secret.
- A payment answers 202 straight away and finishes a moment later. Poll for the end state.
- The first linked bank becomes the receiving account.
