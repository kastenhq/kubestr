ARG BUILDPLATFROM

FROM --platform=$BUILDPLATFORM golang:1.23-bookworm@sha256:7d1571fea7cdc08fe7f37d4eacf06dfe5756366acb7bdd33cf26a7c422fed993 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG TARGETPLATFROM

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS} \
    GOARCH=${TARGETARCH} 

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -o /dist/kubestr -ldflags="-w -s" .

FROM alpine:3.19

RUN apk --no-cache add fio

COPY --from=builder /dist/kubestr /

ENTRYPOINT ["/kubestr"]
