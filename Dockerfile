FROM golang:1.20-alpine AS builder
WORKDIR /go/src/woongkie-talkie
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /woongkie-talkie

FROM alpine:3.18
RUN apk --no-cache add ca-certificates && \
    addgroup -S appgroup && adduser -S appuser -G appgroup
ENV GOPATH=/go
COPY --from=builder /woongkie-talkie /woongkie-talkie
COPY --from=builder /go/src/woongkie-talkie/view /go/src/woongkie-talkie/view
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
CMD ["/woongkie-talkie", "serve"]
