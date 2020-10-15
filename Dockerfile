FROM golang:alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /bin/kubestr

FROM scratch

COPY --from=builder /bin/kubestr /bin/kubestr

ENTRYPOINT ["/bin/kubestr"]
