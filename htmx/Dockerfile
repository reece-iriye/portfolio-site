# ---- Build stage ----
FROM golang:1.24.5-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 go build -o portfolio-server ./

# ---- Runtime stage ----
FROM alpine:edge
WORKDIR /app
COPY --from=builder /app/portfolio-server ./
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
RUN apk --no-cache add ca-certificates tzdata
ENV TZ=UTC
USER developuser
EXPOSE 8080
ENTRYPOINT ["/app/portfolio-server"]
