# Live deployment confirmation (Nobus Cube)

Use this after pushing the finished backend. It proves, in order, that the new build is live, that
the browser can reach it, and that the whole story works on the public URL.

## A. Point the Cube at the right branch

The finished backend is on the branch `APAPO_BACKEND_CLEAN`. In the Cube settings change the branch from
`APAPO_BACKEND` to `APAPO_BACKEND_CLEAN` (or create a new Cube with it), then redeploy or rebuild.
Keep Container Port `8080` and tick "Dockerfile Present".

Environment variables on the Cube:

| Name | Value |
|---|---|
| `APP_SECRET` | a long random string, set once and never changed during the event |
| `ALLOWED_ORIGIN` | the exact frontend origin, for example `https://your-frontend.example`, or `*` while testing |
| `TRUST_PROXY` | `true` |
| `DEV_OTP` | `123456` |
| `PROVIDER` | `mock` |
| `DEMO_ENDPOINTS` | `true` |
| `VERSION` | `0.2.0` |

## B. Five checks, in this order

Replace `$URL` with the Cube URL (https).

1. **The build is live and is the new one.**
   `curl -s $URL/api/health` must show `"status":"ok"` and `"version":"0.2.0"`.
   If the version is not `0.2.0` the Cube is still running the old build: redeploy.
2. **The container is healthy.** In the console the Cube shows running and healthy; `curl -s $URL/healthz` prints `ok`.
3. **The browser can reach it (CORS).** Open the frontend, then the browser developer tools, Network tab. The call to `/api/health` must be green and the badge must say Connected. A red CORS error means `ALLOWED_ORIGIN` does not match the page's exact origin.
4. **The whole story works on the public URL.** From any laptop with curl and python3:
   `./scripts/demo.sh $URL` must end with `Demo complete.` It takes about 30 seconds.
5. **The frontend works end to end.** Sign in as Samuel (08012345678, PIN 4821): the dashboard shows N195,000 and four banks. Send N20,000 to 08098765432 and watch it complete.

All five green means the live deployment is confirmed.

## C. Before every presentation

1. `curl -X POST $URL/api/demo/reset` (or the demo panel button).
2. Sign in again as Samuel and check N195,000.
3. Do not redeploy during the event: it wipes memory and, without a fixed `APP_SECRET`, every token.

## D. If something is wrong

| Symptom | Likely cause | Fix |
|---|---|---|
| Health shows the old version | Cube still on the old branch or not rebuilt | Switch to `APAPO_BACKEND_CLEAN` and redeploy |
| Browser says CORS error | Origin missing in `ALLOWED_ORIGIN` | Exact match: scheme, host and port, no path |
| Mixed content blocked | Frontend is https but the API URL is http | Use the https Cube URL |
| Everything is 401 after a while | Tokens last 24 hours, or `APP_SECRET` was not set and the app restarted | Set `APP_SECRET`, sign in again |
| Payment stays `needs_attention` | A reversal outage switch is still on | `POST /api/demo/failure` with `{"mode":"none"}` |
| Demo script says "cannot reach" | Wrong URL or the Cube is stopped | Check the URL and the Cube status |
| Everything resets | Redeploy or restart (state is in memory) | Expected. Run reset and sign in again |

## E. Honest limits

- One instance only. State is in memory, so never scale to two instances.
- The Docker image build was not run in the build environment (Docker Hub is blocked there). The same Go build was run and tested. The first Cube build is therefore the first real image build: watch its log.
