FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod .
COPY cmd ./cmd
COPY internal ./internal
COPY config ./config
RUN go build -o /rfguard ./cmd/rfguard

FROM alpine:3.19
RUN adduser -D -g '' rfguard
USER rfguard
WORKDIR /app
COPY --from=build /rfguard /app/rfguard
COPY config /app/config
EXPOSE 8080 8081 5514
ENTRYPOINT ["/app/rfguard"]
CMD ["-config", "/app/config/example.yaml"]
