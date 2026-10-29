#!/bin/sh
set -e

docker build -f Dockerfile.test -t go-api-test .
docker run --rm go-api-test