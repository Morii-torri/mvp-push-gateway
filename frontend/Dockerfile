ARG MGP_NODE_BUILDER_IMAGE=node:20-alpine
ARG MGP_FRONTEND_RUNTIME_IMAGE=nginx:1.27-alpine

FROM ${MGP_NODE_BUILDER_IMAGE} AS build

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend /app/frontend
RUN npm run build

FROM ${MGP_FRONTEND_RUNTIME_IMAGE}

COPY docker/frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/frontend/dist /usr/share/nginx/html
