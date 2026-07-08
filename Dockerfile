# ============ 编译阶段 ============
FROM golang:1.25-alpine AS builder

ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=0

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o ginchat .

# ============ 运行阶段 ============
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Shanghai
ENV GINCHAT_ENV=docker

WORKDIR /app
COPY --from=builder /src/ginchat .
COPY --from=builder /src/config ./config
COPY --from=builder /src/docs ./docs
RUN mkdir -p uploads

EXPOSE 8080
CMD ["./ginchat"]