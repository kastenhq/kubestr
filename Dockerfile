FROM golang:1.21-bullseye AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOBIN=/dist

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go install -ldflags="-w -s" .

FROM alpine:3.19

RUN apk --no-cache add fio

COPY --from=builder /dist/kubestr /

ENTRYPOINT ["/kubestr"]
