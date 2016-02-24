#!/bin/bash
readonly IMAGE=$(cat /dev/urandom | tr -cd 'a-f0-9' | head -c 32)
docker build -t "${IMAGE}" -f Dockerfile.build .

readonly CONTAINER=$(docker create "${IMAGE}")
docker cp "${CONTAINER}":/go/bin/app url-shortener
docker rm "${CONTAINER}"
docker rmi "${IMAGE}"
