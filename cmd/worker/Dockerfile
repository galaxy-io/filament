# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/filament/worker /app

USER nonroot:nonroot

ENTRYPOINT ["/app"]
