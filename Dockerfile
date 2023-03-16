ARG BASE_IMAGE=gcr.io/distroless/base-debian11
ARG BUILDER_IMAGE=gcr.io/distroless/static-debian11
FROM ${BUILDER_IMAGE} AS builder

COPY . .

RUN CGO_ENABLED=1 go build -o /go/bin/woongkie-talkie

FROM ${BASE_IMAGE}

ENV GOPATH /go

COPY ./view /go/src/woongkie-talkie/view

COPY --from=builder /go/bin/woongkie-talkie /

EXPOSE 8080

CMD ["/woongkie-talkie", "serve"]