ARG BASE_IMAGE=golang:1.20.2
FROM ${BASE_IMAGE}

WORKDIR /go/src/woongkie-talkie

COPY go.* ./

RUN go mod download