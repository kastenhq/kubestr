ARG BUILDPLATFROM

FROM --platform=$BUILDPLATFORM golang:1.26.2-bookworm@sha256:47ce5636e9936b2c5cbf708925578ef386b4f8872aec74a67bd13a627d242b19 AS builder

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

FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11

RUN apk --no-cache add fio

COPY --from=builder /dist/kubestr /

ENTRYPOINT ["/kubestr"]
