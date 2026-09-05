# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/filament/worker /app

USER 65532:65532

ENTRYPOINT ["/app"]
