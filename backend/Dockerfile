FROM golang:1.22-bookworm AS build

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend /src/backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mgp-server ./cmd/server

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends bash ca-certificates curl postgresql-client \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/mgp-server /app/mgp-server
COPY backend/migrations /app/backend/migrations
COPY scripts /app/scripts

ENV MGP_HOST=0.0.0.0
ENV MGP_PORT=8080
ENV MGP_API_PREFIX=/api/v1

ENTRYPOINT ["/app/scripts/start-backend-container.sh"]
