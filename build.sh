#!/bin/bash

readonly CONTAINER_NAME="lascap/url-shortener"
set -e

TMPDIR=$(mktemp -d)
cp -R public "${TMPDIR}/public"
cp Dockerfile "${TMPDIR}"

echo "Building binary..."
if [ -z "${CIRCLECI}" ]; then
  # `docker rm` doesn't work on CircleCI.
  readonly RM_FLAG="--rm"
fi
mkdir -p release
docker-compose run ${RM_FLAG} -e CGO_ENABLED=0 builder /bin/bash -c "cp /etc/ssl/certs/ca-certificates.crt release && go build -ldflags \"-s\" -a -installsuffix cgo -o release/url-shortener"
cp release/url-shortener release/ca-certificates.crt "${TMPDIR}"

echo "Packaging Docker image..."
if [ -n "$1" ]; then
  readonly TAG="${CONTAINER_NAME}:${1}"
else
  # Using "latest".
  readonly TAG="${CONTAINER_NAME}"
fi
docker build --build-arg GIT_SHA1 -t "${TAG}" "${TMPDIR}"

rm -rf "${TMPDIR}"
