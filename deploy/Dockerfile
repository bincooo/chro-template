FROM golang:1.20-alpine AS builder

WORKDIR /app
RUN apk add git && git clone https://github.com/bincooo/chro-template.git .
RUN go mod tidy && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o linux-server -trimpath

FROM alpine:3.19.0
WORKDIR /app
COPY --from=builder /app/linux-server ./linux-server
COPY --from=builder /app/config.yaml ./config.yaml
RUN chmod +x linux-server

ENTRYPOINT ["sh","-c","./linux-server"]