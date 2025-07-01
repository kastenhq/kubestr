ARG BUILDPLATFROM

FROM --platform=$BUILDPLATFORM golang:1.24-bookworm@sha256:8faa537d0690017d8000e38e1ed9ed4c5990d40a3022ce2596a825c6c87f62c4 AS builder

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

FROM alpine:3.22@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715

RUN apk --no-cache add fio

COPY --from=builder /dist/kubestr /

ENTRYPOINT ["/kubestr"]
