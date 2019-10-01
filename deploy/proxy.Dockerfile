FROM golang:alpine AS builder

WORKDIR /home/app/

ADD . .

RUN go build --mod=vendor -o proxy /home/app/cmd/proxy/main.go

FROM bashell/alpine-bash

WORKDIR /home/app/

COPY . .

COPY --from=builder /home/app/proxy /home/app/