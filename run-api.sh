#!/bin/sh

docker build -t go-api .
docker run --rm -p 8080:8080 go-api