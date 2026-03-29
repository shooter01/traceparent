FROM golang:1.25 AS build
WORKDIR /src
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/app main.go

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/app /app/app
EXPOSE 8080
ENTRYPOINT ["/app/app"]
