# syntax=docker/dockerfile:1
FROM gcr.io/distroless/static:nonroot

COPY bin/filament-control /app

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["/app"]
