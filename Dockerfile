FROM golang:alpine AS builder

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN go build -ldflags="-w -s" -o kubestr .

WORKDIR /dist

RUN cp /app/kubestr .

FROM alpine:3.9

COPY --from=builder /dist/kubestr /

ENTRYPOINT ["/kubestr"]
