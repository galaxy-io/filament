# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/filament/control-plane /app

EXPOSE 8081
USER nonroot:nonroot

ENTRYPOINT ["/app"]
