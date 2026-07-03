# syntax=docker/dockerfile:1

# Build context is the repo root:
#   docker build -f cmd/ingestion-control/Dockerfile -t filament-control .

ARG GO_VERSION=1.26.4

FROM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src
COPY . .

ENV GOWORK=off CGO_ENABLED=0

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    cd cmd/ingestion-control && go build -trimpath -buildvcs=false -ldflags="-s -w" -o /out/app .

FROM gcr.io/distroless/static:nonroot AS runtime

COPY --from=build /out/app /app

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app"]
