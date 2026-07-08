# build both linux binaries into bin/
binaries:
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/ingestion-control -trimpath -ldflags="-s -w" -o ../../bin/filament-control .
    GOWORK=off CGO_ENABLED=0 GOOS=linux go build -C cmd/ingestion-worker -trimpath -ldflags="-s -w" -o ../../bin/filament-worker .

# build both docker images
images: binaries
    docker build -f cmd/ingestion-control/Dockerfile -t galaxy-io/filament:latest .
    docker build -f cmd/ingestion-worker/Dockerfile -t galaxy-io/filament-worker:latest .
