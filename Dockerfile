# Multi-stage build for Go application
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o malnutrition-app main.go

# Production minimal container
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/malnutrition-app /app/malnutrition-app
COPY templates/ /app/templates/

ENV PORT=8080
ENV PYTHON_API_URL=http://127.0.0.1:8000

EXPOSE 8080

CMD ["/app/malnutrition-app"]
