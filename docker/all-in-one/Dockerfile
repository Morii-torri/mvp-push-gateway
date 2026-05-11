ARG MGP_GO_BUILDER_IMAGE=golang:1.22-alpine
ARG MGP_NODE_BUILDER_IMAGE=node:20-alpine
ARG MGP_ALL_IN_ONE_RUNTIME_IMAGE=postgres:16-alpine

FROM ${MGP_GO_BUILDER_IMAGE} AS backend-build

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend /src/backend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mgp-server ./cmd/server \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mgp-migrate ./cmd/migrate \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/mgp-healthcheck ./cmd/healthcheck

FROM ${MGP_NODE_BUILDER_IMAGE} AS frontend-build

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend /src/frontend
RUN npm run build

FROM ${MGP_ALL_IN_ONE_RUNTIME_IMAGE}

RUN apk add --no-cache ca-certificates nginx

WORKDIR /app
COPY --from=backend-build /out/mgp-server /app/mgp-server
COPY --from=backend-build /out/mgp-migrate /app/mgp-migrate
COPY --from=backend-build /out/mgp-healthcheck /app/mgp-healthcheck
COPY --from=frontend-build /src/frontend/dist /usr/share/nginx/html
COPY backend/migrations /app/backend/migrations
COPY docker/all-in-one/entrypoint.sh /usr/local/bin/mgp-all-in-one-entrypoint.sh
COPY docker/all-in-one/nginx.conf /etc/nginx/http.d/default.conf

ENV MGP_HOST=0.0.0.0
ENV MGP_PORT=8080
ENV MGP_API_PREFIX=/api/v1
ENV MGP_MIGRATIONS_DIR=/app/backend/migrations
ENV PGDATA=/var/lib/postgresql/data

EXPOSE 80
VOLUME ["/var/lib/postgresql/data"]
ENTRYPOINT ["/usr/local/bin/mgp-all-in-one-entrypoint.sh"]
