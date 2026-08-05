# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/filament/metrics /app

EXPOSE 8081
USER nonroot:nonroot

ENTRYPOINT ["/app"]
