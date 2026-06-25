# ZENLENET PingMesh - 多阶段构建, 产出单文件镜像
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG GIT_COMMIT=docker
ARG BUILD_TIME=unknown
# 纯 Go SQLite 驱动, 无需 CGO, 支持交叉编译; 注入版本信息
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o /out/pingmesh ./src

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata wget
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /out/pingmesh /app/pingmesh
# 前端资源与默认配置已内嵌于二进制, 首次启动自动释放到 /app
VOLUME ["/app/conf", "/app/db", "/app/logs"]
EXPOSE 8899
# 容器健康检查: 命中 /healthz 即视为存活
HEALTHCHECK --interval=30s --timeout=4s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8899/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["/app/pingmesh"]
