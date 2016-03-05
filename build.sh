#!/bin/bash
readonly CONTAINER_NAME="lascap/url-shortener"
readonly HASH="$(cat /dev/urandom | tr -cd 'a-f0-9' | head -c 10)"
readonly IMAGE="${CONTAINER_NAME}-build:${HASH}"
docker build -t "${IMAGE}" -f Dockerfile.build .

docker run --rm -v /var/run/docker.sock:/var/run/docker.sock -e CONTAINER_TAG="${CONTAINER_NAME}:${HASH}" "${IMAGE}"

docker rmi "${IMAGE}"
