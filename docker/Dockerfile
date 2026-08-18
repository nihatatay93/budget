FROM node:24-alpine AS web-build

WORKDIR /src
COPY apps/web/package.json apps/web/package-lock.json ./apps/web/
RUN cd apps/web && npm ci
COPY apps/web ./apps/web
RUN mkdir -p apps/server/internal/webui/dist \
    && cd apps/web \
    && npm run build

FROM golang:1.26-alpine AS server-build

WORKDIR /src/apps/server
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download
COPY apps/server ./
COPY --from=web-build /src/apps/server/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -tags=production -trimpath -o /out/budget ./cmd/server

FROM alpine:3.23

RUN apk add --no-cache ca-certificates \
    && addgroup -S budget \
    && adduser -S -G budget budget

COPY --from=server-build /out/budget /usr/local/bin/budget

USER budget
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/budget"]
