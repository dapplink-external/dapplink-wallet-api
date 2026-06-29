FROM golang:1.25-alpine3.24 as builder

RUN apk add --no-cache make ca-certificates gcc musl-dev linux-headers git jq bash

COPY ./go.mod /app/go.mod
COPY ./go.sum /app/go.sum

WORKDIR /app

RUN go mod download

ARG CONFIG=config.yml

# build dapplink-wallet-api with the shared go.mod & go.sum files
COPY . /app/dapplink-wallet-api

WORKDIR /app/dapplink-wallet-api

RUN make

FROM alpine:3.18

COPY --from=builder /app/dapplink-wallet-api/wallet-api /usr/local/bin
COPY --from=builder /app/dapplink-wallet-api/${CONFIG} /etc/dapplink-wallet-api/

WORKDIR /app

ENTRYPOINT ["wallet-api"]
CMD ["-c", "/etc/dapplink-wallet-api/config.yml"]