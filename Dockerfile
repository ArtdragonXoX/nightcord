# 构建阶段
FROM golang:latest as builder

ENV GOPROXY=https://goproxy.cn,direct
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

RUN apt-get update \
    && apt-get install -y build-essential git \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY . .
RUN go build -o build/nightcord main.go

# 导出阶段（仅包含编译产物）
FROM scratch as export
COPY --from=builder /app/build/nightcord /