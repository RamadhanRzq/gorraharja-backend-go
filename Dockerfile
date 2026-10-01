# syntax=docker/dockerfile:1

# ---- Build stage -----------------------------------------------------------
FROM golang:1.27-alpine AS builder

WORKDIR /src

# Cache module downloads independently of the source tree.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binary: no libc dependency, so the distroless static image works.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/api ./cmd/api

# ---- Runtime stage ---------------------------------------------------------
# distroless/static-debian12 ships no shell and no package manager. The :nonroot
# tag runs as UID/GID 65532 by default.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/api /api

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/api"]
