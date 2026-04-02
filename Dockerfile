# 1단계: 빌드
FROM golang:1.22-alpine AS builder

WORKDIR /build

# 의존성 먼저 캐시 (코드 변경 시 재다운로드 방지)
COPY go.mod go.sum ./
RUN go mod download

# 소스 복사 후 빌드
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o nfs-csi-driver ./cmd/

# 2단계: 실행 이미지 (최소화)
FROM alpine:3.19

# NFS 마운트에 필요한 패키지
RUN apk add --no-cache nfs-utils

COPY --from=builder /build/nfs-csi-driver /usr/local/bin/nfs-csi-driver

ENTRYPOINT ["/usr/local/bin/nfs-csi-driver"]