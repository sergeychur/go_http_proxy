FROM golang:alpine AS builder

WORKDIR /home/app/

ADD . .

RUN go build --mod=vendor -o repeater /home/app/cmd/repeater/main.go

FROM bashell/alpine-bash

WORKDIR /home/app/

COPY . .

COPY --from=builder /home/app/repeater /home/app