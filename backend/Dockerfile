ARG MGP_GO_BUILDER_IMAGE=golang:1.22-alpine
ARG MGP_BACKEND_RUNTIME_IMAGE=alpine:3.20

FROM ${MGP_GO_BUILDER_IMAGE} AS build

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend /src/backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mgp-server ./cmd/server \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mgp-migrate ./cmd/migrate \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mgp-healthcheck ./cmd/healthcheck

FROM ${MGP_BACKEND_RUNTIME_IMAGE}

RUN apk add --no-cache ca-certificates \
  && addgroup -S mgp \
  && adduser -S -D -H -G mgp mgp

WORKDIR /app
COPY --from=build /out/mgp-server /app/mgp-server
COPY --from=build /out/mgp-migrate /app/mgp-migrate
COPY --from=build /out/mgp-healthcheck /app/mgp-healthcheck
COPY backend/migrations /app/backend/migrations

ENV MGP_HOST=0.0.0.0
ENV MGP_PORT=8080
ENV MGP_API_PREFIX=/api/v1
ENV MGP_MIGRATIONS_DIR=/app/backend/migrations

EXPOSE 8080
USER mgp:mgp
CMD ["/app/mgp-server"]
