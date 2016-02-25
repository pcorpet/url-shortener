#!/bin/bash
readonly IMAGE=$(cat /dev/urandom | tr -cd 'a-f0-9' | head -c 32)
docker build -t "${IMAGE}" -f Dockerfile.build .
docker run --rm "${IMAGE}" go test
docker rmi "${IMAGE}"

