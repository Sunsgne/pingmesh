# ZENLENET PingMesh - 多阶段构建, 产出单文件镜像
FROM golang:1.22-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /out/pingmesh ./src

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
WORKDIR /app
COPY --from=build /out/pingmesh /app/pingmesh
# 前端资源与默认配置已内嵌于二进制, 首次启动自动释放到 /app
VOLUME ["/app/conf", "/app/db", "/app/logs"]
EXPOSE 8899
ENTRYPOINT ["/app/pingmesh"]
