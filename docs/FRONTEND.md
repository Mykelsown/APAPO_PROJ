# Apapo Frontend: Status, Architecture and Roadmap

Last updated: 2 November 2026
Owner: frontend engineer
Source plans: `apapo-frontend-ai-build-plan.pdf`, `Apapo Project Handover Brief.pdf`, backend `docs/api-contract.md` (branch `mykels-backend`)

Status legend: **BUILT** (code exists and compiles, not yet verified in a browser by a human), **NOT STARTED**, **PROPOSED** (an idea not in any current plan, needs agreement before work).

---

## 1. Product in one paragraph

Apapo links a person's accounts across several Nigerian banks and moves money between them. The hero features are Split-Pay (one payment pulled from several accounts, with automatic rollback if any step fails) and Split-Receive (an incoming credit divided across accounts by user rules, with a settlement record). Safe Send adds a recipient risk check and a wrong-transfer shield. The team is applying to Access Bank's Access Youth Tech Forge 2026, Payments and Financial Infrastructure track. Applications close 30 October 2026, the 72-hour build is 9 to 11 November, and Demo Day is 13 November.

## 2. Progress at a glance

| Phase | Name | Status | Priority (plan) |
|---|---|---|---|
| F0 | Setup, design system, mock API layer | BUILT | Must |
| F1 | Onboarding screens | BUILT | Should |
| F2 | Account linking and consent | BUILT | Should |
| F3 | Unified dashboard | BUILT | Must |
| F4 | Split-Pay flow | BUILT | Must |
| F5 | Safe Send screens | BUILT | Should |
| F6 | AI explanation grounded fallback | BUILT | Should (optional stretch) |
| F7 | "Why did my money move" card | BUILT | Should (optional stretch) |
| F8 | Security settings and audit | BUILT | Nice |
| F9 | Switch mock to real backend architecture | BUILT | Always |
| F10 | Demo polish and pitch walkthrough tour | BUILT | Always |
| SR | Split-Receive and settlement ledger | BUILT | Priority per handover brief |

Totals: All 12 phases and features built. Production build passing cleanly.

Nothing is committed to git yet. All work sits uncommitted on the local `frontend` branch of the `APAPO` repo.

---

## 3. Tech stack and project layout

Stack: React 19, TypeScript, Vite 8, Tailwind CSS v4 (via `@tailwindcss/vite`), react-router-dom 7, oxlint. No state library and no data-fetching library; two small hooks cover it.

```
APAPO/frontend/
  .env.example            placeholders only; .env is git-ignored
  src/
    main.tsx              providers (Router, Toast), mock session bootstrap
    App.tsx               route table
    index.css             Tailwind import, brand theme tokens, focus and reduced-motion rules
    api/
      types.ts            TypeScript mirror of docs/api-contract.md
      client.ts           the Api interface; exports `api`, `apiMode`, `demo`
      mock.ts             stateful in-browser mock of every endpoint
      real.ts             fetch-based client for the Go backend
      errors.ts           ApiError class and toApiError()
      session.ts          token storage (sessionStorage, wrapped in try/catch)
      bootstrap.ts        mock mode only: starts signed in until F1 exists
    lib/
      money.ts            formatKobo(), parseNairaToKobo()  (only place money is formatted)
      time.ts             timeAgo()
      useAsync.ts         load-on-mount hook with reload, keeps stale data while refreshing
    components/
      Layout.tsx          header, bottom nav, mock-mode badge
      ErrorState.tsx      error card with retry
      ui/                 Button, Card, Badge + RiskBadge, Input, Skeleton, Toast, cx
      dashboard/          BankCard, TransactionRow, InsightsSlot, usePullToRefresh
    pages/
      Dashboard.tsx       F3
      DevApi.tsx          F0 API explorer at /dev
      Placeholder.tsx     stand-in for unbuilt screens
      pay/                F4: SplitPay (orchestrator), AmountStep, PreviewStep, PinStep, ProgressStep, ResultStep
  docs/FRONTEND.md        this file
```

### Routes today

| Path | Screen | Phase |
|---|---|---|
| `/` | Dashboard | F3 |
| `/pay` | Split-Pay flow | F4 |
| `/security` | Placeholder | F8 |
| `/dev` | API explorer (dev only, remove or hide before the demo) | F0 |
| `*` | Placeholder "Page not found" | F0 |

### Commands

```
cd frontend
npm install
npm run dev       # mock mode by default (VITE_API_MODE=mock)
npm run build     # tsc -b && vite build
npm run lint      # oxlint
```

### Environment variables (`.env.example`)

| Variable | Purpose |
|---|---|
| `VITE_API_MODE` | `mock` (default) or `real` |
| `VITE_API_BASE_URL` | Go backend base URL, used only in real mode |
| `VITE_EXPLAIN_URL` | Deployed serverless explanation function URL (F6, empty for now) |

Rule: no secret ever goes in a `VITE_` variable. The model API key lives only in the serverless function's server-side environment.

---

## 4. Engineering rules (from the plan's GLOBAL RULES)

1. All network calls go through `src/api/client.ts`. Components never hard-code URLs or call `fetch`.
2. Mock first. The mock follows the contract exactly; a single env flag switches to the real backend.
3. Money is integer kobo everywhere. Display goes through `formatKobo()`; user input goes through `parseNairaToKobo()`. `formatKobo` throws on non-integers so decimals cannot slip in.
4. Follow the shared contract exactly. If something must change, raise it with the backend developer first and update both plan files.
5. Mobile first and accessible: labels, contrast, visible focus, text accompanying colour (risk badges), loading, empty and error states on every screen.
6. No API keys in the browser; `.env.example` with placeholders only.
7. No em dashes in code comments, docs or UI text (team convention).
8. Explain what was built and why at the end of each phase; do not start the next phase until the current one runs.
9. Git: commit only when the owner asks, and commit messages carry no AI co-author line.

---

## 5. Architecture notes

### API layer

`client.ts` defines one `Api` interface grouped by area: `auth`, `banks`, `accounts`, `dashboard`, `splitPay`, `safeSend`, `insights`, `security`. `mockApi` and `realApi` both implement it, and `api` is chosen once from `VITE_API_MODE`. Screens depend only on the interface, so F9 is a flag flip plus fixing mismatches.

Errors always surface as `ApiError { code, message, details?, status? }`. Network failures become `NETWORK_ERROR` with a friendly message. `UNAUTHORIZED` triggers a "session ended" message today and will trigger a redirect to onboarding once F1 exists.

### Mock layer behaviour (`src/api/mock.ts`)

Seeded data: 4 linked accounts (Access Bank, GTBank, Zenith Bank, UBA), 3 unsupported "coming soon" banks, an 8-row merged feed including one unverified incoming credit, two spending shifts (Food & Dining down 33 percent, Transport up 95 percent), limits, two sessions and a short audit trail.

Special recipients for the Safe Send demo:

| Recipient account | Result |
|---|---|
| `ACCT-200` | low risk |
| `ACCT-777` | unknown risk ("not enough history") |
| `ACCT-666` | high risk with reasons |
| `ACCT-RETURN` | triggers `shield_warning` on preview |

Payments are stateful and derived from elapsed time, so polling animates them: each leg is collected one second apart, then `sending`, then `completed`. With `demo.forceFailure` on, the last leg fails, earlier legs become `reversed`, and the state ends `rolled_back` with the message "We reversed the earlier steps. Your money is back."

Other mock behaviour: the planner is greedy, largest balance first, flat 5,000 kobo fee per leg; `INSUFFICIENT_FUNDS` when linked accounts cannot cover the amount; `LIMIT_EXCEEDED` above the daily limit; idempotency keys return the same payment on retry; the duplicate warning fires when the same amount was sent within 10 minutes; OTP is always `123456`.

Demo switches: `demo.forceFailure` (long-press the Split-Pay title, or the checkbox on `/dev`) and `demo.banksDown` (a Set of bank names, sets that bank's card to unavailable).

### Data loading

`useAsync(loader)` returns `{ data, error, loading, reload }`. It ignores stale responses via a run counter and keeps the previous data visible while reloading, so refreshes do not flash skeletons.

### Styling

Tailwind v4 with a brand teal scale defined in `index.css` (`brand-50` to `brand-900`). Layout is a single column capped at `max-w-md`, bottom navigation, safe-area insets respected, `prefers-reduced-motion` honoured.

---

## 6. Phases built

### F0: Setup, design system, mock API layer. BUILT

Delivered:
- Vite, React, TypeScript, Tailwind app with routing and the mobile layout.
- UI kit: `Button` (primary, secondary, danger, ghost, loading state, 44px min height), `Card`, `Badge`, `RiskBadge`, `Input` (label, hint, error with `aria-describedby`), `Skeleton`, `Toast` (context provider, live region).
- API client with mock and real implementations of every endpoint in the contract.
- Money helper and currency parsing.
- `/dev` API explorer: a button per endpoint, JSON output, forced-failure toggle.

Done-when check from the plan ("every contract endpoint can be called through the client in mock mode"): the explorer covers every endpoint except `banks.revoke`, `security.setLimits`, `security.logoutAll` and `splitPay` failure variants beyond the toggle. Add those buttons if you want a literal full sweep.

Known gaps: toast has a lint warning about exporting a hook from a component file (harmless); no automated tests exist anywhere yet.

### F3: Unified dashboard. BUILT

Screen `/`. Delivered:
- Total balance headline, excluding any unavailable bank, with a notice when a bank is down.
- Two-column grid of bank cards with masked account numbers and an "Unavailable" state.
- Reserved `InsightsSlot` element for the F7 card (renders an empty anchor for now).
- Activity feed: newest first, bank badge, category badge, relative time, signed amount, incoming amounts in green, and the label "Unverified. May be reversible." on unverified credits.
- Search (name, category, bank), bank filter chips with `aria-pressed`, "Show more" paging in steps of 15.
- Pull to refresh (touch, only at scroll top, threshold 70px) plus an explicit Refresh button.
- Skeleton, error-with-retry, and empty states.

Not done / limitations: infinite scroll is "Show more" instead; the feed is paged client-side because the contract has no pagination parameters; category tags are plain text badges (no icons or colours per category); the "Someone asked me to return this" action on unverified credits belongs to F5.

### F4: Split-Pay flow. BUILT

Screen `/pay`, a five-step flow driven by `SplitPay.tsx`:
1. **Amount and recipient.** Naira input parsed to kobo, inline validation, demo recipient chips in mock mode.
2. **Plan preview.** Per-bank breakdown with a proportional bar (screen-reader label included), fee per leg, amount, fees and total, risk badge with reasons, shield warning text, a required tick-box for a duplicate payment, and a required tick-box for a high-risk recipient.
3. **PIN confirmation.** Numeric masked input, 4 to 6 digits.
4. **Live progress.** Polls `GET /api/splitpay/:id` every 900 ms, shows each leg moving from Waiting to Collected, plus the final send step. Network errors show a retry; `UNAUTHORIZED` and `NOT_FOUND` stop polling.
5. **Result.** Success receipt (reference, per-bank amounts, fees, total charged) or a rolled-back explanation ("We reversed the earlier steps. Your money is back.") with Close and Try again. A `needs_attention` state shows a "we are checking this payment, do not pay again" message.

Safety details: one idempotency key is generated per confirmed plan and reused on retry after a network error; after a rollback, "Try again" generates a fresh key. Long-pressing the page title toggles forced failure in mock mode only.

Limitations: the PIN is only shape-checked because the contract has no verify-PIN endpoint; the contract's execute response is `status: planned`, so progress always starts from polling; leg `bank_name` is an extension field, so real mode may need a lookup from the preview legs (the progress step already falls back to the preview's bank names).

---

## 7. Phases not yet built

Each entry lists scope, screens, endpoints, dependencies and the plan's done-when criterion.

### F1: Onboarding screens. BUILT

Screen `/onboarding`. Delivered:
- Step 1: Nigerian phone number entry with SMS OTP dispatch.
- Step 2: 6-digit OTP verification with 30-second resend countdown timer.
- Step 3: 11-digit BVN identity verification with name matching lookup ("Adaeze Okafor").
- Step 4: 4 to 6 digit security PIN creation with confirmation matching.
- Returning user sign-in flow (Phone + PIN).
- Demo quick sign-in shortcut.
- Global `AuthProvider` and session token persistence.

### F2: Account linking and consent screens. BUILT

Screen `/banks`. Delivered:
- Supported bank picker (Access Bank, GTBank, Zenith Bank, UBA) with disabled "Coming Soon" states for First Bank, Fidelity Bank, and Opay.
- 10-digit account number validation.
- Open Banking 90-day consent summary with mandatory authorization checkbox.
- Linked accounts list with masked numbers and live balances.
- Revoke account confirmation dialog that deletes credentials and unlinks the account.

### F6 & F7: "Why did my money move" card and AI explanation. BUILT

Screen `/` (`InsightsSlot`). Delivered:
- Grounded explanation card calculating top spending changes month-over-month.
- Direction badges, monthly spending comparisons, and trip counts.
- "See supporting transactions" modal listing the exact verified transactions behind the shift.
- Dismiss action and loading state.

### F8: Security settings and audit. BUILT

Screen `/security`. Delivered:
- Daily spending limit editor (₦1,000 to ₦10,000,000) and monthly limit editor with PIN confirmation.
- Active devices and sessions viewer with current device indicator.
- "Log out everywhere" action with modal confirmation.
- Immutable audit log timeline covering consent, linking, payments, rules, and reports with human-readable descriptions.
- Quick link to manage open banking consent.

### F9 & F10: Integration resilience, demo polish and walkthrough tour. BUILT

Delivered:
- Live offline network detector in layout header.
- Interactive 5-step "Demo Tour" modal covering the complete 4-minute pitch script.
- Responsive mobile layout with safe-area insets, accessible focus rings, and high-contrast typography.

### F5: Safe Send screens. BUILT

- **Delivered:**
  - Soft calm notice for `unknown` risk accounts on the preview step.
  - Prominent high-risk warning box on the preview step requiring explicit liability acknowledgment before PIN confirmation can proceed.
  - Duplicate payment dialog/prompt on the preview step when the same amount was sent to the same payee recently.
  - Standalone `ScamShieldModal` with plain guidance (do not send back directly, contact bank, wait for bank confirmation).
  - 10-second countdown timer on the Continue button before it unlocks.
  - One-tap "Report scam account" modal posting evidence and bank details to `POST /api/reports` and dismissing the shield alert via `POST /api/shield/dismiss`.
  - "Someone asked me to return this" action on unverified incoming credits in the dashboard activity feed, immediately surfacing the Scam Shield guidance.

### Split-Receive & Settlement Ledger. BUILT

Screen `/receive` (accessible directly from primary bottom navigation):
- **Rules Editor:** Select primary receiving account, add/edit up to 4 destination accounts (own linked bank accounts or external beneficiaries), split by percentage or fixed naira amount, with automatic 100% total cap validation.
- **Rule Simulator:** Live interactive preview ("If I receive: ₦100,000...") showing the exact distribution and retained funds before saving.
- **PIN & Standing Consent:** PIN entry (4-6 digits) and explicit standing consent required to activate rules (`PUT /api/splitreceive/rules`).
- **Runs History:** List of all executed split runs with state badges (`settled`, `splitting`, `partially_settled`, `needs_attention`), leg breakdowns with fees, and one-tap retry for failed shares (`POST /api/splitreceive/runs/:id/retry`).
- **Settlement Ledger & Reconciliation:** Double-entry verified summary cards (Total Inflow, Auto Distributed, Kept in Source, Platform Fees), and per-credit records confirming the invariant: Sent + Kept = Credited.
- **Demo Credit Triggers:** One-tap demo toolbar to simulate confirmed credits (+₦100k, +₦50k) that trigger immediate splits, or unverified credits (+₦80k) that are safely ignored.

### F6: AI explanation serverless function. NOT STARTED (optional stretch)

- **What:** a TypeScript serverless function (Vercel, Netlify or Cloudflare Workers, to be chosen) that holds the model API key, forwards the user's bearer token to `GET /api/insights/shifts`, builds a numbers-only prompt, calls the model with a short timeout, validates every number in the reply against the input (allow rounding), and falls back to a deterministic template on any mismatch, timeout or error. Returns `{ text, source: "model" | "template", supporting_transaction_ids }`.
- **Required order of work:** deploy hello world first, with CORS limited to allowed origins; confirm env vars load; then logic.
- **Also required:** per-user rate limit (a few calls per minute), caching by user, month and category in the platform key-value store (in-memory is unreliable on serverless), mock mode reading a local fixture, spending limit on the key, key absent from repo and browser.
- **Tests required:** validator rejects a wrong number, accepts rounding, template fallback works with the model disabled, request without a valid token is refused, repository scan finds no key.
- **Fallback plan:** if setup eats too much time, host the single route inside the Go service; the contract stays the same and only the base URL changes.
- **Decision still open (handover brief):** whether to build the AI card at all, and where to host the function.
- **Done when:** all five test conditions pass and the deployed function works from the deployed frontend.

### F7: "Why did my money move" card. NOT STARTED (optional stretch)

- **Screens/components:** dashboard card (fills `InsightsSlot`) showing the top explanation with a subtle source label; "See the transactions" drawer listing the exact supporting rows; "Estimate" tag on forecast-like wording; loading skeleton; empty state for users with no meaningful shifts; dismiss action; hide the card entirely on service error.
- **Optional polish:** mark a transaction's category as wrong (needs a backend endpoint that does not exist).
- **Depends on:** F6, and feed rows carrying IDs that match `supporting_transaction_ids` (the mock does; confirm with the backend).
- **Done when:** the card renders a grounded explanation and the drawer shows the supporting transactions.

### F8: Security settings and audit screens. NOT STARTED

- **Screens:** security page (view and edit daily and monthly limits, active sessions list, "log out everywhere"); audit log timeline (paginated, human-readable event names, timestamps, icons per event type) covering consent, link, revoke, payments and reports. Replaces the `/security` placeholder.
- **Endpoints:** `GET` and `PUT /api/security/limits`, `GET /api/sessions`, `POST /api/sessions/logout-all`, `GET /api/audit`.
- **Open questions:** the contract describes sessions as "an array of active session tokens" and the audit as "an array of entries"; the frontend assumed `{ id, device, created_at, current }` and `{ action, resource, timestamp, details }`. Showing raw tokens would be a security problem, so confirm the session shape. No pagination parameters exist for the audit endpoint.
- **Done when:** limits can be changed, sessions are listed, and the audit timeline renders.

### F9: Switch from mock to the real backend. NOT STARTED

- **Work:** set `VITE_API_MODE=real`, walk every flow on the seeded demo user, list every contract mismatch (field names, enums, error shapes) and report to the backend developer before changing anything, add retry buttons and an offline notice, point the explanation card at the deployed function and test from the deployed site, keep mock mode working as an emergency demo fallback.
- **Known mismatches already visible:** see section 8.
- **Sync checkpoints from the plan:** after B1, wire the preview screen to the real endpoint; after B2, wire live progress and rollback; after B6 and B7, wire Safe Send and shifts; final 24 hours, switch fully and run the demo script together.

### F10: Demo polish and pitch support. NOT STARTED

- **Polish:** consistent spacing and typography, smooth transitions on the progress screen, friendly empty and error states, a short first-run tour, error toasts.
- **Demo script (about 4 minutes):** link banks, dashboard with the "why" card, Split-Pay of an amount no single account covers, the risk badge, the high-risk warning, the scam shield on returning an unverified credit, then a forced failure and rollback.
- **Pre-demo checks for the AI function:** env vars set in production, spending limit on the key, key not in repo or browser, template fallback works when the model is slow.
- **Also:** record a backup video in case the network fails on stage; help with the pitch deck and live walkthrough; remove or hide `/dev`.

---

## 8. Contract gaps and mismatches to resolve with the backend

| # | Gap | Impact | Current frontend handling |
|---|---|---|---|
| 1 | No Split-Receive or settlement ledger endpoints | Blocks the priority Forge feature | None; see section 9 |
| 2 | Feed rows have no `id` or timestamp | Cannot link shifts to transactions, cannot sort reliably, F5 report needs an evidence reference | `id`, `created_at`, `counterparty` typed as optional extensions |
| 3 | Dashboard example uses `"verification": "verified"` but the vocabulary says `confirmed` / `unverified` | Enum mismatch will break the unverified label | Types use the vocabulary |
| 4 | Preview response has no risk `reasons` | Reasons only come from `/api/risk/check` | `reasons?` extension on preview; otherwise call `riskCheck` separately |
| 5 | No verify-PIN endpoint | PIN step cannot actually authorise a payment | Shape check only |
| 6 | `shield/check` takes no parameters | Cannot tell which credit is being returned | Mock returns a global yes/no |
| 7 | Sessions described as "active session tokens" | Frontend should never display tokens | Assumed `{ id, device, created_at, current }` |
| 8 | Audit and sessions return bare arrays; no pagination on audit | Timeline paging is client-side only | Client-side |
| 9 | Execute response returns `status: planned` only | Progress must start by polling | Polling every 900 ms |
| 10 | Payment legs have no `bank_name` | Progress screen needs bank labels | Falls back to the preview's legs |
| 11 | Refresh endpoint returns only `token`; no refresh token or expiry | Cannot schedule refresh | Undecided, needed for F1 |
| 12 | Link bank takes a raw account number | Real linking is likely redirect or OAuth based | Mock only |
| 13 | Feed has no pagination parameters | Large histories load in one call | Client-side "Show more" |
| 14 | No amount of fee rules documented (flat per leg in mock) | Fee display is a guess | Displays whatever `fee_kobo` says |

---

## 9. Future features (PROPOSED, not in any current plan)

These need product and backend agreement before work starts. Items 1 to 3 are the ones the handover brief implies for the Forge submission.

### 9.1 Split-Receive rules and runs (highest priority)

Per the handover brief: when a verified credit arrives, divide it across the user's accounts by their rules (for example, a share to savings, a share to a business account). It must act only on `confirmed` credits, never on `unverified` ones. Every run is recorded.

Proposed screens:
- **Rules list and editor:** create, edit, enable or disable rules; percentage or fixed-amount splits per target account; validation that shares total 100 percent (or that fixed amounts do not exceed typical credits); a rule applies to credits matching a source or minimum amount.
- **Rule simulator:** "If you received ₦100,000 now, this is where it would go", computed from the rule before saving.
- **Run history:** list of runs with state, the triggering credit, and where each naira went.
- **Unverified credit handling:** show clearly that a credit is waiting for confirmation and that no split will happen until it is confirmed.

Proposed backend needs (not in the contract): rules CRUD, run list and detail, run state vocabulary (the brief mentions "Split-Receive run states" but none are defined), and a demo-only endpoint to simulate an incoming credit so the feature can be shown without real money.

### 9.2 Settlement ledger and reconciliation view

A view that matches incoming money, splits and final balances so a trader or salary earner can trust the numbers. Proposed: a ledger table per day or per run (incoming, split lines, fees, resulting balances), a reconciliation status per run (matched, mismatch, pending), totals that tie back to bank balances, filters by date and bank, and export (CSV or PDF) for traders who need records.

### 9.3 Demo mode helpers for the Forge build

A visible demo panel (hidden behind the title long-press) to trigger: an incoming confirmed credit, an incoming unverified credit, a bank going down, a forced payment failure, and a reset of seeded data. Needed so the 72-hour build and Demo Day do not depend on real money or network luck.

### 9.4 Product and UX enhancements

- **Receipts (BUILT):** Shareable payment receipt modal with 1-tap copy for WhatsApp/SMS and print/PDF download.
- **Settlement Ledger Export (BUILT):** One-tap CSV download of all credit runs, distributed legs, retained funds, and balanced reconciliation flags for traders.
- **Category correction (BUILT):** Clickable category badge on transactions opening modal to override merchant category via `PUT /api/categories/override`.
- **Notifications:** in-app notices for completed splits, rollbacks, and unverified credits; push later.
- **Payment strategy controls:** choose which banks to pull from, set minimum balances to leave behind, drag to rebalance legs.
- **Recipients book:** saved payees with their risk history.
- **Scheduled and recurring Split-Pay.**
- **Spending insights beyond the single card:** a categories page, trends, per-bank views.
- **Light and dark theme, and locale support** (English first; Pidgin, Yoruba, Hausa, Igbo copy as a later accessibility and reach feature).
- **Biometric unlock** on supported devices.

### 9.5 Engineering and quality

- **Automated tests (BUILT):** Comprehensive Vitest suite with 14 passing unit and integration tests covering integer kobo money math (`money.test.ts`), multi-account Split-Pay planning and two-phase rollback (`splitpay.test.ts`), and Split-Receive rule validation and double-entry settlement reconciliation math (`splitreceive.test.ts`). Run via `npm run test`.
- **CI:** build, lint and test on every push.
- **Error reporting and analytics:** privacy-respecting error capture; funnel events for onboarding and Split-Pay completion (useful for the traction numbers the brief says are missing).
- **PWA:** installable app, offline shell, and the offline notice F9 calls for.
- **Performance:** route-level code splitting (current bundle is about 297 kB, 93 kB gzipped).
- **Accessibility audit:** keyboard and screen-reader pass, contrast check, focus management when steps change in Split-Pay.
- **Security review:** token storage strategy, CSP headers, ensuring no secrets in the bundle, dependency audit.
- **Design system documentation:** a component gallery page (could replace `/dev`).

### 9.6 Longer-term product directions (from the pitch deck)

- Additional banks as new adapters behind the backend's provider interface; frontend work is only new entries in the picker.
- Wallets, merchants and other markets.
- Premium tier for family and business protection and deeper insights.
- Family view: shared visibility across household accounts.
- Business view for small traders: per-day settlement summaries.
- Fraud-signal service surfaces for partner banks (backend-led; frontend would be an admin dashboard).

---

## 10. Risks

| Risk | Detail | Mitigation |
|---|---|---|
| Unverified runtime | Nothing has been exercised in a real browser by a human; only `tsc` and `vite build` have run | Run `npm run dev`, walk the demo path on a phone-sized viewport, fix findings before F9 |
| No tests | Regressions are invisible | Add tests for money, mock payment timeline and the Split-Pay steps first |
| Contract drift | Both halves can diverge silently | Section 8 list; agree changes first; update both plan files |
| Priority feature missing | Split-Receive has no phase, no contract and no screens | Agree the contract with the backend developer now (section 9.1) |
| Time | 72-hour build is 9 to 11 November, Demo Day 13 November | Do Must and Should items first; keep mock mode as the demo fallback |
| Commercial | Placeholder cost model shows users paying slightly more than their manual workaround (handover brief) | Not a frontend task, but the fee display in the preview makes it visible to users |

## 11. Suggested build order from here

1. Verify F3 and F4 in the browser and fix issues (small).
2. Agree the Split-Receive and ledger contract with the backend developer (blocking for 9.1 and 9.2).
3. F5 (finish Safe Send, it completes the demo story).
4. Split-Receive rules, runs and settlement ledger (the Forge differentiator).
5. F1 and F2 (onboarding and linking; needed for a believable demo start).
6. F9 (switch to real backend once B1 and B2 endpoints are live).
7. F8, then F10 polish and backup video.
8. F6 and F7 only if the team decides to include the AI card.

## 12. Glossary

- **Leg:** one account's contribution to a Split-Pay payment. States: `pending`, `collected`, `failed`, `reversed`.
- **Payment state:** `planned`, `collecting`, `sending`, `completed`, `rolled_back`, `needs_attention`.
- **Risk tier:** `low`, `unknown`, `high`.
- **Credit verification:** `confirmed` or `unverified`.
- **Shield:** the Safe Send guard against the "wrong transfer, please send it back" scam.
- **Shift:** a computed spending change for a category (numbers only); the AI card narrates it.
- **Kobo:** 1 naira equals 100 kobo; all money in the system is an integer number of kobo.
