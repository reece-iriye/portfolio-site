#!/usr/bin/env bash

docker compose -f compose.dev.yaml pull
docker compose -f compose.dev.yaml build
docker compose -f compose.dev.yaml up -d

