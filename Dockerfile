FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build

FROM alpine
RUN apk add --no-cache git
COPY --from=builder /app/static-wiki-editor /static-wiki-editor
ENTRYPOINT ["/static-wiki-editor"]
