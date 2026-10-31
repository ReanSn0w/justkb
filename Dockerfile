# syntax=docker/dockerfile:1.7
FROM node:24-alpine AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci
COPY index.html tsconfig.json vite.config.ts eslint.config.js .prettierrc.json ./
COPY src ./src
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/internal/frontend/dist ./internal/frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/jkb ./cmd/jkb && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/jkb-migrate ./cmd/jkb-migrate && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/jkb-admin ./cmd/jkb-admin

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S -g 10001 jkb && adduser -S -D -H -u 10001 -G jkb jkb
WORKDIR /app
COPY --from=backend /out/jkb /out/jkb-migrate /out/jkb-admin /usr/local/bin/
COPY --from=backend /src/migrations ./migrations
RUN mkdir -p /var/lib/jkb/attachments && chown -R jkb:jkb /var/lib/jkb
USER jkb
EXPOSE 8080
VOLUME ["/var/lib/jkb/attachments"]
CMD ["/usr/local/bin/jkb"]
