#!/usr/bin/env bash
# Walks the whole Apapo story against a running backend with two users.
# Usage: scripts/demo.sh [base_url]     (default http://localhost:8080)
# Needs: bash, curl, python3. Resets demo data first, so it is safe to run again and again.
set -u
BASE="${1:-http://localhost:8080}"
BASE="${BASE%/}"
STATUS=0
BODY=""
step=0

say() { step=$((step + 1)); printf '\n\033[1m%02d. %s\033[0m\n' "$step" "$*"; }
die() { printf '\033[31mFAILED: %s\033[0m\n' "$*" >&2; printf 'last status %s body %s\n' "$STATUS" "$BODY" >&2; exit 1; }
ok()  { printf '   ok: %s\n' "$*"; }

# api METHOD PATH [TOKEN] [JSON_BODY] [IDEMPOTENCY_KEY]  -> sets STATUS and BODY
api() {
  local method="$1" path="$2" token="${3:-}" data="${4:-}" key="${5:-}"
  local args=(-sS -X "$method" -H 'Content-Type: application/json' -o /tmp/apapo_demo_body -w '%{http_code}')
  [ -n "$token" ] && args+=(-H "Authorization: Bearer $token")
  [ -n "$key" ] && args+=(-H "Idempotency-Key: $key")
  [ -n "$data" ] && args+=(-d "$data")
  STATUS=$(curl "${args[@]}" "$BASE$path") || die "cannot reach $BASE$path"
  BODY=$(cat /tmp/apapo_demo_body)
}
# j 'python expression on d' : read from BODY
j() { printf '%s' "$BODY" | python3 -c "import sys,json; d=json.load(sys.stdin); print($1)"; }
expect_status() { [ "$STATUS" = "$1" ] || die "expected HTTP $1 but got $STATUS"; }
expect_eq() { [ "$1" = "$2" ] || die "$3: expected '$2' but got '$1'"; }
naira() { python3 -c "print(f'N{$1/100:,.0f}')"; }

wait_state() { # wait_state TOKEN PAYMENT_ID WANTED SECONDS
  local i
  for i in $(seq 1 "$4"); do
    api GET "/api/splitpay/$2" "$1"
    [ "$(j "d['state']")" = "$3" ] && return 0
    sleep 1
  done
  die "payment $2 did not reach $3 in $4 seconds (state $(j "d['state']"))"
}
total() { api GET /api/dashboard "$1"; j "d['total_balance_kobo']"; }

say "Backend is reachable"
api GET /api/health; expect_status 200
expect_eq "$(j "d['status']")" ok "health status"
ok "$(j "d['service']") $(j "d['version']") provider=$(j "d['provider']")"

say "Reset demo data and sign in both users"
api POST /api/demo/reset; expect_status 200
api POST /api/auth/login "" '{"phone":"08012345678","pin":"4821"}'; expect_status 200
SAM=$(j "d['access_token']")
api POST /api/auth/login "" '{"phone":"08098765432","pin":"5912"}'; expect_status 200
BOLA=$(j "d['access_token']")
ok "Samuel and Bola are signed in"

say "Samuel's unified dashboard"
api GET /api/dashboard "$SAM"; expect_status 200
expect_eq "$(j "d['total_balance_kobo']")" 19500000 "Samuel total"
ok "total $(naira 19500000) across $(j "len(d['banks'])") banks; $(j "sum(1 for t in d['transactions'] if t['verification']=='unverified')") unconfirmed credit in the feed"

say "Bola links two more banks and sets her Split-Receive rules (30% savings, 40% spending)"
api POST /api/banks/link "$BOLA" '{"bank_code":"delta","consent":{"accepted":true,"scopes":["accounts","balances","transactions"]}}'; expect_status 201
api POST /api/banks/link "$BOLA" '{"bank_code":"savanna","consent":{"accepted":true,"scopes":["accounts","balances","transactions"]}}'; expect_status 201
api GET /api/accounts "$BOLA"; expect_status 200
APEX=$(j "[a['id'] for a in d['accounts'] if a['bank_code']=='apex'][0]")
DELTA=$(j "[a['id'] for a in d['accounts'] if a['bank_code']=='delta'][0]")
SAV=$(j "[a['id'] for a in d['accounts'] if a['bank_code']=='savanna'][0]")
api PUT /api/splitreceive/rules "$BOLA" "{\"receiving_account_id\":\"$APEX\",\"standing_consent\":true,\"pin\":\"5912\",\"destinations\":[{\"account_id\":\"$SAV\",\"percent\":30,\"label\":\"Savings\"},{\"account_id\":\"$DELTA\",\"percent\":40,\"label\":\"Spending\"}]}"
expect_status 200
ok "rules saved"

say "Samuel looks up Bola by phone and sends her N20,000 (peer to peer)"
api GET "/api/users/lookup?phone=08098765432" "$SAM"; expect_status 200
expect_eq "$(j "d['user']['display_name']")" "Bola A." "masked name"
ok "found '$(j "d['user']['display_name']")' (masked, no account number)"
api POST /api/splitpay/preview "$SAM" '{"amount_kobo":2000000,"recipient":{"apapo_phone":"08098765432","recipient_type":"apapo","masked_display_name":"Bola A."}}'; expect_status 200
ok "preview accepted the frontend display fields; risk tier $(j "d['risk']['tier']"), fee $(naira "$(j "d['total_fee_kobo']")")"
api POST /api/splitpay/execute "$SAM" '{"amount_kobo":2000000,"recipient":{"apapo_phone":"08098765432"},"pin":"4821"}' "demo-p2p-1"; expect_status 202
PAY=$(j "d['id']"); wait_state "$SAM" "$PAY" completed 20
ok "payment $PAY completed"

say "Bola's rules split that credit once, and the ledger balances"
api GET /api/splitreceive/runs "$BOLA"; expect_status 200
expect_eq "$(j "len(d['runs'])")" 1 "runs after P2P"
expect_eq "$(j "d['runs'][0]['sent_kobo']")" 1400000 "sent share"
api GET /api/reconciliation "$BOLA"; expect_status 200
expect_eq "$(j "all(c['balanced'] for c in d['credits'])")" True "every credit balanced"
ok "N20,000 in: $(naira "$(j "d['totals']['sent_kobo']")") sent, $(naira "$(j "d['totals']['kept_kobo']")") kept, $(naira "$(j "d['totals']['held_kobo']")") held"

say "A simulated N200,000 salary arrives for Bola and splits 30/40"
api POST /api/demo/simulate-credit "$BOLA" '{"amount_kobo":20000000,"sender_name":"Acme Payroll"}'; expect_status 201
expect_eq "$(j "d['runs'][0]['sent_kobo']")" 14000000 "salary sent"
expect_eq "$(j "d['runs'][0]['kept_kobo']")" 6000000 "salary kept"
api GET /api/reconciliation "$BOLA"; expect_status 200
expect_eq "$(j "d['totals']['sent_kobo']+d['totals']['held_kobo']+d['totals']['kept_kobo']==d['totals']['credit_kobo']")" True "ledger invariant"
ok "N140,000 sent (N60,000 savings, N80,000 spending), N60,000 kept. Invariant sent+held+kept=credit holds"

say "Safe Send flags the reported account"
api POST /api/risk/check "$SAM" '{"amount_kobo":500000,"recipient":{"bank_code":"apex","account_number":"9000000001"}}'; expect_status 200
expect_eq "$(j "d['risk']['tier']")" high "risk tier"
ok "tier high: $(j "d['risk']['reasons'][0]")"
api POST /api/splitpay/execute "$SAM" '{"amount_kobo":500000,"recipient":{"bank_code":"apex","account_number":"9000000001"},"pin":"4821"}' "demo-flagged-1"
expect_status 409
ok "execute refused without acknowledge_risk (409 $(j "d['error']['code']")), no money moved"

say "A forced failure rolls the payment back and restores every balance"
BEFORE=$(total "$SAM")
SAMACC=$(api GET /api/accounts "$SAM" && j "[a['id'] for a in d['accounts'] if a['bank_code']=='savanna'][0]")
api POST /api/demo/failure "" "{\"account_id\":\"$SAMACC\",\"mode\":\"insufficient_funds\"}"; expect_status 200
api POST /api/splitpay/execute "$SAM" '{"amount_kobo":15000000,"recipient":{"bank_code":"delta","account_number":"1000000001"},"pin":"4821"}' "demo-rollback-1"; expect_status 202
PAY=$(j "d['id']"); wait_state "$SAM" "$PAY" rolled_back 20
expect_eq "$(j "[l['state'] for l in d['legs']]")" "['reversed', 'reversed', 'failed']" "leg states"
expect_eq "$(total "$SAM")" "$BEFORE" "balance after rollback"
ok "rolled_back, earlier legs reversed, total back to $(naira "$BEFORE")"

say "Reversal outage: the money is held, retried, then rolled back"
api POST /api/demo/failure "" '{"bank_code":"delta","mode":"reversal_down"}'; expect_status 200
api POST /api/splitpay/execute "$SAM" '{"amount_kobo":15000000,"recipient":{"bank_code":"delta","account_number":"1000000001"},"pin":"4821"}' "demo-hold-1"; expect_status 202
PAY=$(j "d['id']")
wait_state "$SAM" "$PAY" needs_attention 10
ok "needs_attention, money safe in the collector account, attempts so far: $(j "d['reversal_attempt_count']")"
api POST /api/demo/failure "" '{"bank_code":"delta","mode":"none"}'; expect_status 200
ok "outage cleared, waiting for the background worker (up to 30 seconds)"
wait_state "$SAM" "$PAY" rolled_back 40
expect_eq "$(total "$SAM")" "$BEFORE" "balance after the retried rollback"
api POST /api/demo/failure "" '{"mode":"none"}'; expect_status 200
api GET "/api/splitpay/$PAY" "$SAM"
ok "rolled_back after $(j "d['reversal_attempt_count']") failed attempt(s) and one that worked"

say "A clean N150,000 payment to an outside account completes (three legs)"
api POST /api/splitpay/execute "$SAM" '{"amount_kobo":15000000,"recipient":{"bank_code":"delta","account_number":"1000000001"},"pin":"4821"}' "demo-150k-1"; expect_status 202
PAY=$(j "d['id']"); wait_state "$SAM" "$PAY" completed 20
ok "completed with $(j "len(d['legs'])") legs, collector $(j "[l['bank_name'] for l in d['legs'] if l['is_collector']][0]"), fees $(naira "$(j "d['total_fee_kobo']")")"
AFTER=$(total "$SAM")
ok "Samuel's total is now $(naira "$AFTER")"

say "Shield warns about sending on money the bank has not confirmed"
api GET "/api/shield/check?amount_kobo=8000000" "$SAM"; expect_status 200
expect_eq "$(j "d['alert'] is not None")" True "shield alert"
ok "$(j "d['alert']['message']")"

say "History and recent recipients"
api GET "/api/payments?limit=10" "$SAM"; expect_status 200
ok "$(j "len(d['payments'])") payments in Samuel's history, newest is $(j "d['payments'][0]['state']")"
api GET /api/recipients/recent "$SAM"; expect_status 200
ok "$(j "len(d['recipients'])") recent recipients, all masked"

say "Privacy: Bola cannot read Samuel's payments, and lookups never show account numbers"
api GET "/api/splitpay/$PAY" "$BOLA"; expect_status 404
ok "Bola gets 404 for Samuel's payment"
api GET /api/payments "$BOLA"; expect_status 200
expect_eq "$(j "len(d['payments'])")" 0 "Bola's payment list"
api GET "/api/users/lookup?phone=08098765432" "$SAM"
case "$BODY" in *2009009009*|*Adeyemi*) die "lookup leaked private data";; esac
ok "lookup returned only a masked name"

printf '\n\033[1;32mDemo complete.\033[0m\n'
