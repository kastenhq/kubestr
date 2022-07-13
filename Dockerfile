FROM golang:1.17.12-alpine3.16 AS builder

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

RUN go get -ldflags="-w -s" .

FROM alpine:3.16

RUN apk --no-cache add fio

COPY --from=builder /dist/kubestr /

ENTRYPOINT ["/kubestr"]
