# ---------- build ----------
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -o scheduler ./cmd/scheduler

# ---------- runtime ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/scheduler /app/scheduler

USER nonroot:nonroot

ENTRYPOINT ["/app/scheduler"]
