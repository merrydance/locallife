# Build stage
FROM golang:1.24-alpine3.20 AS builder
WORKDIR /app
COPY . .
RUN go build -o locallife-rest main.go

# Run stage
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/locallife-rest .
COPY app.env .
COPY start.sh .
COPY wait-for.sh .
COPY db/migration ./db/migration

EXPOSE 8080 9090
CMD [ "/app/locallife-rest" ]
ENTRYPOINT [ "/app/start.sh" ]
