# Endpoint log

One plain-words report per finished endpoint, newest last. Forward to the frontend dev.

## GET /api/health and GET /healthz

- **Who may call**: anyone, no sign in.
- **Send**: nothing.
- **`GET /api/health` returns** 200 with JSON: `status` ("ok"), `service` ("apapo-api"), `version`, `provider` ("mock"), `time` (ISO UTC), `message` ("Backend is reachable").
- **`GET /healthz` returns** 200 with the plain text `ok`. Meant for Docker and uptime checks, not the app.
- **Errors**: a wrong method gets 405 `method_not_allowed`; an unknown path gets 404 `not_found`; a browser preflight from an origin not in `ALLOWED_ORIGIN` gets 403 `origin_not_allowed`. If the server is down, the request fails at the network level (no JSON at all).
- **Frontend must**: call `/api/health` on app load and show "Connected" only when the call succeeds and `status` is "ok". Treat a network error or timeout as "Not connected". Do not show version or time to users unless useful for debugging. Do not use `/healthz` from the app.
