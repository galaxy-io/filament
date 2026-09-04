# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/filament/standalone /app

EXPOSE 8080
USER 65532:65532

ENTRYPOINT ["/app"]
