FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies first so they're cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

# Static binary on a minimal, non-root base image.
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /api /api

EXPOSE 8080
ENTRYPOINT ["/api"]
