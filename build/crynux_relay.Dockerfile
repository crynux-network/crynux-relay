FROM golang:1.24-alpine3.23 AS builder
RUN apk add --no-cache --update gcc g++

WORKDIR /crynux_relay

COPY go.* .

RUN CGO_ENABLED=1 go mod download

COPY . .

RUN CGO_ENABLED=1 go build

FROM alpine:3.23

RUN apk add --no-cache tzdata \
    && addgroup -S -g 10001 crynux \
    && adduser -S -u 10001 -G crynux crynux

ENV TZ=Asia/Tokyo

WORKDIR /app

COPY --from=builder --chown=crynux:crynux /crynux_relay/crynux_relay .
COPY --chown=crynux:crynux config/gpu_flops.json ./config/gpu_flops.json
COPY --chown=crynux:crynux static ./static

USER crynux:crynux

CMD ["/app/crynux_relay"]
