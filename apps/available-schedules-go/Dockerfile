FROM golang:1.24 AS build
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o available-schedules-go

FROM gcr.io/distroless/base-debian12
WORKDIR /srv
COPY --from=build /app/available-schedules-go /srv/available-schedules-go
ENV PORT=8080 \
    SERVICE_NAME=available-schedules \
    ERROR_RATE=0.01 \
    EXTRA_LATENCY_MS=0
EXPOSE 8080
ENTRYPOINT ["/srv/available-schedules-go"]

