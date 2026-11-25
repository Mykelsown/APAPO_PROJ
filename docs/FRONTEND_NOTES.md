# Notes for the frontend

Short, plain notes per endpoint. The full table is in `docs/api-contract.md`.

## General

- Money is integer kobo. Divide by 100 only when displaying. Never send naira decimals.
- Every error has `error.code` (stable, switch on this) and `error.message` (safe to show).
- Local dev: either set `ALLOWED_ORIGIN=http://localhost:5173` on the backend, or proxy `/api` from the dev server to `http://localhost:8080`.
