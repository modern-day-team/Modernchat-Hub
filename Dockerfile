# 生产镜像：前端构建 + Go 后端，单容器部署。
# 数据库仍使用外部 MySQL；JWT_SECRET 等敏感配置通过环境变量/secret 注入。
FROM node:22-alpine AS web-build
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci --no-audit --no-fund
COPY web/ .
RUN npm run build

FROM golang:1.26-alpine AS go-build
WORKDIR /app/server
RUN apk add --no-cache git
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X github.com/modern-day-team/Modernchat-Hub/server/internal/version.Version=${VERSION} \
      -X github.com/modern-day-team/Modernchat-Hub/server/internal/version.Commit=${COMMIT} \
      -X github.com/modern-day-team/Modernchat-Hub/server/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/modernchat-hub-server ./cmd/server

FROM alpine:3.21
WORKDIR /opt/modernchat-hub
RUN apk add --no-cache ca-certificates tzdata \
    && mkdir -p /opt/modernchat-hub/bin /opt/modernchat-hub/server /opt/modernchat-hub/web /opt/modernchat-hub/deploy
COPY --from=go-build /out/modernchat-hub-server /opt/modernchat-hub/bin/modernchat-hub-server
COPY --from=web-build /app/web/dist /opt/modernchat-hub/web/dist
COPY server/.env.example /opt/modernchat-hub/server/.env.example
COPY deploy/ /opt/modernchat-hub/deploy/
EXPOSE 8091
ENTRYPOINT ["/opt/modernchat-hub/bin/modernchat-hub-server"]
