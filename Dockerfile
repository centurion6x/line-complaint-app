# syntax=docker/dockerfile:1

# ---- Frontend build stage -------------------------------------------------
FROM node:22-alpine AS frontend-build
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
# VITE_LIFF_ID is baked into the JS bundle at BUILD time (Vite inlines
# import.meta.env.* during the build) — it cannot be changed at container
# runtime. Pass it as a Docker build argument. See the deployment guide.
ARG VITE_LIFF_ID
ENV VITE_LIFF_ID=${VITE_LIFF_ID}
RUN npm run build

# ---- Backend build stage ---------------------------------------------------
FROM golang:1.24-alpine AS backend-build
WORKDIR /app/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/server .

# ---- Runtime ----------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=backend-build /out/server ./server
COPY --from=frontend-build /app/frontend/dist ./static

ENV STATIC_DIR=/app/static
ENV PORT=8080
# LINE_CHANNEL_ID, LINE_CHANNEL_ACCESS_TOKEN and PUBLIC_BASE_URL are
# required at runtime and are intentionally NOT set here — provide them
# as environment variables on your deployment platform. The process
# fails fast at startup if any are missing.

USER app
EXPOSE 8080
ENTRYPOINT ["./server"]
