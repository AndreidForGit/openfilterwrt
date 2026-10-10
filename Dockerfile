# syntax=docker/dockerfile:1

# ---------- Etapa de compilación ----------
FROM golang:1.26.8-bookworm AS builder

ARG TARGETOS=linux
ARG TARGETARCH=amd64

ENV CGO_ENABLED=0
ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV GOTOOLCHAIN=local

WORKDIR /source

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build \
        -trimpath \
        -ldflags="-s -w" \
        -o /output/openwrt-controller \
        .

# ---------- Etapa de ejecución ----------
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /output/openwrt-controller /usr/local/bin/openwrt-controller
COPY --from=builder /source/static /app/static

ENV LISTEN_ADDRESS=":8080"
ENV STATIC_DIRECTORY="/app/static"

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/openwrt-controller"]

