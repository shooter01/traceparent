FROM golang:1.25 AS build
WORKDIR /src
COPY . .
ARG SERVICE
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/app ./cmd/${SERVICE}

FROM alpine:3.20
WORKDIR /app
COPY --from=build /out/app /app/app
EXPOSE 8080 8081
ENTRYPOINT ["/app/app"]