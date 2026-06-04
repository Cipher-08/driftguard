# ---- build stage ----
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache deps first
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/driftguard ./cmd/api

# ---- runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 driftguard
USER driftguard
WORKDIR /app
COPY --from=build /out/driftguard /app/driftguard
EXPOSE 8080
ENTRYPOINT ["/app/driftguard"]
