# Deployment Guide — LINE Complaint App

## Overview

This app is a single Docker image containing a Go backend (which also
serves the built React/LIFF frontend as static files) — one container,
one port, one URL. There is no database and no authentication system:
identity is recovered per-request from the LIFF session's ID token, and
uploaded photos live only in an in-memory, TTL-based cache inside the
running process. Read the whole guide before deploying — secrets,
Console setup and Coolify setup all depend on each other.

## Secrets & configuration

| Variable | Scope | Where it's used | Notes |
|---|---|---|---|
| `VITE_LIFF_ID` | **Build-time** (frontend) | Baked into the JS bundle by Vite when the image is built | Must be passed as a Docker `--build-arg` / Coolify "Build Variable" — setting it as a normal runtime env var has **no effect**, since the frontend is already compiled by the time the container starts. |
| `LINE_CHANNEL_ID` | Runtime (backend) | Expected `aud` when verifying LIFF ID tokens | This is the **LINE Login (or LINE MINI App) channel's** Channel ID — the channel the LIFF app itself is attached to. It is **not** the Messaging API channel ID (see below). Not secret, but keep consistent with the Console. |
| `LINE_CHANNEL_ACCESS_TOKEN` | Runtime (backend) | Bearer token for the Messaging API push endpoint | **Secret.** Treat like a password — store it as a Coolify secret, never commit it, rotate it if it leaks. |
| `PUBLIC_BASE_URL` | Runtime (backend) | Builds the public HTTPS image URLs that LINE's servers fetch for image messages | Must be the real public HTTPS origin the app is reachable at once deployed (no trailing slash). Must start with `https://` — the server refuses to start otherwise. |
| `PORT` | Runtime (backend) | Port the Go server listens on | Defaults to `8080`; usually left alone on Coolify, which proxies to it. |

No other secrets are required — there is no database, no session store,
no third-party auth provider.

> **Important: you need two channels, not one.** LINE no longer allows
> LIFF apps to be added to a Messaging API channel directly — LIFF must
> be hosted on a **LINE Login** (or **LINE MINI App**) channel instead
> ([LINE's announcement](https://developers.line.biz/en/news/2019/11/11/liff-cannot-be-used-with-messaging-api-channels/)).
> This still works for this app's design: as long as both channels sit
> under the **same Provider**, LINE issues the same user ID (`sub`) for
> a given person across every channel under that Provider. So the
> backend's ID-token verification (against the Login channel) and its
> push call (via the Messaging API channel) resolve to the same user —
> there's just no single channel that does both anymore.

## LINE Developers Console setup

1. In the [LINE Developers Console](https://developers.line.biz/console/),
   create (or reuse) a **Provider** — note its name, you'll put both
   channels under it.
2. Under that Provider, create a **Messaging API** channel (this is your
   Official Account / bot). On its **Messaging API** tab, issue a
   long-lived **Channel access token** — this is your
   `LINE_CHANNEL_ACCESS_TOKEN`.
3. Under the **same Provider**, create a second channel: a **LINE Login**
   channel (choose app type "Web app"), or a **LINE MINI App** channel if
   you'd rather follow LINE's newer recommended path for LIFF. This
   second channel is what will host the LIFF app itself — channel names
   can't contain "LINE" or similar strings.
4. On that Login/MINI App channel, note its **Channel ID** from **Basic
   settings** — this is your `LINE_CHANNEL_ID` (used only to verify ID
   tokens; it has nothing to do with the push call).
5. Still on that same Login/MINI App channel, open the **LIFF** tab and
   add a LIFF app: set its **Endpoint URL** to your deployed app's URL
   (you'll come back and update this once you know your real domain),
   and its **Size** to `Full` for the best form-filling experience. Note
   the generated **LIFF ID** — this is your `VITE_LIFF_ID`.
6. Back on the Messaging API channel, disable auto-reply/greeting
   messages you don't want interfering with the flow (this app doesn't
   use webhooks at all — it only pushes messages out, so webhook
   settings can stay off).
7. Add a **Rich Menu** to the Messaging API channel (under its Messaging
   API tab, or via the Rich Menu API) with a tap area that opens your
   LIFF app's URL (`https://liff.line.me/<your-liff-id>`) — this is the
   "block UI element" entry point into the complaint form.
8. Have the account you'll test with **add the Messaging API channel's
   Official Account as a friend** (scan its QR code from the Messaging
   API tab, or use its OA link). LINE's push API can only deliver to
   users who have added the OA as a friend — the complaint form will
   still submit successfully for anyone, but the acknowledgement push
   will silently fail to reach a non-friend.

## Deploying on Coolify

1. Push this repository to a Git provider Coolify can reach (GitHub,
   GitLab, etc.) — or add it as a local Git source if your Coolify
   instance supports that.
2. In Coolify, create a new **Application** resource, pointing at this
   repo, and select the **Dockerfile** build pack (not Nixpacks) — the
   repo's root `Dockerfile` does the multi-stage build.
3. Under the application's **Environment Variables**, add
   `LINE_CHANNEL_ID`, `LINE_CHANNEL_ACCESS_TOKEN`, and `PUBLIC_BASE_URL`
   as normal runtime variables (mark `LINE_CHANNEL_ACCESS_TOKEN` as a
   secret if Coolify offers that toggle).
4. Add `VITE_LIFF_ID` as well, but mark it **"Build time: Available
   during build" / "Runtime: Not available"** (or whatever your Coolify
   version calls the build-arg toggle) — this is what makes Coolify pass
   it as `--build-arg VITE_LIFF_ID=...` instead of a plain env var. If
   you only set it as a runtime variable, the build will bake in an
   empty string and LIFF will fail to initialize in production.
5. Set `PUBLIC_BASE_URL` to the domain you're about to assign this
   application in Coolify (you need to decide the domain before the
   first real deploy, since it has to match on both sides).
6. Assign a domain to the application in Coolify and let it issue a
   Let's Encrypt certificate (Coolify's Traefik integration handles this
   automatically) — the app must be served over HTTPS for both LIFF and
   LINE's image-fetching to work.
7. Deploy. Watch the build logs for the frontend build stage completing
   with `VITE_LIFF_ID` actually present (Coolify logs show the
   `--build-arg` being passed).
8. Once deployed, go back to the LINE Developers Console's LIFF tab and
   update the Endpoint URL to your real Coolify domain, and update the
   Rich Menu's tap-area URL to match.

## Operational caveats

- **No persistence, by design.** Uploaded photos are cached in memory
  only (2-hour TTL) so they can be served back to LINE's servers as
  fetchable URLs during a push. They are never written to disk or a
  database. A restart or redeploy clears the cache — in-flight
  complaints submitted right before a restart could lose their images
  if LINE hasn't fetched them yet, though in practice the fetch happens
  within seconds of the push call.
- **Single-replica only.** Because the image cache lives in the
  process's memory, this app cannot be horizontally scaled behind a
  load balancer with more than one replica — a photo cached by one
  replica wouldn't be visible to a request served by another. Keep
  Coolify's replica count at 1.
- **No database at all.** There is no record of complaints anywhere
  once the acknowledgement message is sent — if you need any kind of
  complaint history, reporting, or follow-up workflow later, that's a
  deliberate gap this version doesn't address.
- **No authentication beyond the LIFF session.** Anyone who opens the
  LIFF link while logged into LINE can submit a complaint "as
  themselves" (their real LINE user ID, verified via the ID token) —
  there's no additional login, and no admin view. This matches the "no
  auth for now" requirement, but means anyone with the link can use it.
- **The user must have added the Official Account as a friend.** This
  is a LINE platform rule, not something this app enforces: the push
  endpoint can only deliver messages to users who've added the OA. If
  they haven't, `handleCreateComplaint` already surfaces this as a
  failed submission — the push call errors, the backend logs it, and
  the browser gets back a 502 with "your complaint was captured but the
  chat acknowledgement failed to send." The photos the user uploaded
  are still cached in memory at that point but nothing will ever
  consume them, so make sure your LIFF Endpoint URL instructions (or
  the Rich Menu tap area's surrounding text) tell first-time users to
  add the OA as a friend before opening the form.
- **Rate limits.** The Messaging API push endpoint has its own rate
  limits per channel; this app does not implement any throttling of its
  own on top of that.
- **Image message limits.** LINE requires image URLs for image messages
  to be fetchable over HTTPS with a reasonable response time — if your
  Coolify instance is slow to respond right after a cold start, the very
  first complaint after a restart could occasionally fail to deliver its
  images (the text acknowledgement would still succeed).

## Post-deploy checklist

- [ ] `https://<your-domain>/healthz` returns `{"status":"ok"}`
- [ ] Opening the Rich Menu's tap area (or the raw
      `https://liff.line.me/<liff-id>` link) inside LINE opens the form
- [ ] Submitting a complaint with no photos produces a Flex Message
      acknowledgement in the chat
- [ ] Submitting a complaint with 1–4 photos produces the Flex Message
      **and** the images, in the same chat
- [ ] Submitting more than 4 photos, or an oversized photo, is rejected
      client-side with a clear error
- [ ] The LIFF app's Endpoint URL in the Console matches the deployed
      domain exactly (including `https://`)
- [ ] `PUBLIC_BASE_URL` matches the deployed domain exactly
- [ ] `LINE_CHANNEL_ACCESS_TOKEN` is stored as a secret, not visible in
      build logs or a public repo

## Updating and rotating

- **Changing `VITE_LIFF_ID`, or any frontend code** requires a full
  redeploy (rebuild) — a restart alone reuses the already-built image
  and won't pick it up, since it's compiled into the JS bundle.
- **Changing `LINE_CHANNEL_ID`, `LINE_CHANNEL_ACCESS_TOKEN`, or
  `PUBLIC_BASE_URL`** only requires a restart, since the Go binary reads
  them from the environment at process start — Coolify will typically do
  this automatically when you update a runtime variable and redeploy,
  but a plain restart is enough if the image itself hasn't changed.
- **Rotating the channel access token**: issue a new long-lived token in
  the Console's Messaging API tab, update `LINE_CHANNEL_ACCESS_TOKEN` in
  Coolify, restart. The old token can then be revoked from the Console.
- **Changing the domain**: update it in Coolify, update
  `PUBLIC_BASE_URL` to match, and update the LIFF app's Endpoint URL in
  the Console — all three need to agree, or image messages and the LIFF
  session check will both break.
