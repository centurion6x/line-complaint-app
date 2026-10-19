# LINE Complaint App

A minimal complaint-intake app for a LINE Official Account: a Rich Menu
button opens a LIFF web form (title, description, up to 4 photos); on
submit, a Go backend pushes a Flex Message acknowledgement plus the
photos back into the user's chat.

No database, no auth beyond the LIFF session itself. See
**[DEPLOYMENT.md](./DEPLOYMENT.md)** for the full setup, required
secrets, and Coolify deployment steps — read that before deploying.

## Structure

```
backend/     Go API (stdlib net/http only, no external deps)
frontend/    React + Vite + @line/liff
Dockerfile   Multi-stage build: frontend -> backend -> single runtime image
```

## How it works

1. A Rich Menu tap area (or any link) opens the LIFF app.
2. The frontend calls `liff.init()`; inside LINE the user is already
   logged in.
3. On submit, the frontend gets a fresh ID token (`liff.getIDToken()`)
   and POSTs `multipart/form-data` (title, description, photos,
   idToken) to `POST /api/complaints`.
4. The backend verifies the ID token against LINE's `/oauth2/v2.1/verify`
   endpoint (checked against the LIFF app's own LINE Login channel —
   see DEPLOYMENT.md, LIFF can't live on a Messaging API channel
   anymore) to recover the caller's LINE user ID, caches the photos
   in memory (not on disk, not in a database — see DEPLOYMENT.md for
   why that's a deliberate limitation), and pushes a Flex Message +
   image messages to that user via the Messaging API push endpoint.
5. The backend also serves the built frontend as static files, so the
   whole thing is one container, one port, one URL.

## Local development

You'll need a real LINE channel to actually test the LIFF/LINE parts —
see DEPLOYMENT.md for that setup. For just iterating on code:

```bash
# Terminal 1 — backend
cd backend
export LINE_CHANNEL_ID=... LINE_CHANNEL_ACCESS_TOKEN=... PUBLIC_BASE_URL=https://your-tunnel.example.com
go run .

# Terminal 2 — frontend
cd frontend
cp .env.example .env   # fill in VITE_LIFF_ID
npm install
npm run dev
```

The LIFF SDK requires HTTPS and will not run from a plain
`http://localhost`. Use a tunnel (ngrok, cloudflared) pointed at the
Vite dev server (which proxies `/api` and `/images` to the Go backend
on :8080 — see `vite.config.js`), and set that tunnel's URL as the
LIFF app's endpoint URL in the LINE Developers Console while you test.

## Production build

```bash
docker build --build-arg VITE_LIFF_ID=<your-liff-id> -t complaints-app .
docker run -p 8080:8080 \
  -e LINE_CHANNEL_ID=... \
  -e LINE_CHANNEL_ACCESS_TOKEN=... \
  -e PUBLIC_BASE_URL=https://your-deployed-domain.example.com \
  complaints-app
```
