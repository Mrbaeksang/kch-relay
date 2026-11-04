FROM node:24-bookworm-slim AS frontend
WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-bookworm AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates gosu && rm -rf /var/lib/apt/lists/* && useradd --uid 10001 --create-home app && mkdir -p /data/uploads && chown -R app:app /data
WORKDIR /app
COPY --from=backend /out/server ./server
COPY --from=frontend /src/frontend/dist ./frontend/dist
ENV PORT=8080 DATABASE_PATH=/data/market.db WEB_DIR=/app/frontend/dist
COPY scripts/entrypoint.sh /usr/local/bin/relay-entrypoint
RUN chmod 755 /usr/local/bin/relay-entrypoint
ENTRYPOINT ["relay-entrypoint"]
EXPOSE 8080
CMD ["./server"]
