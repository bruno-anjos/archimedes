#!/bin/bash

set -e

env CGO_ENABLED=0 GOOS=linux go build -o archimedes .
docker build -t brunoanjos/archimedes:latest .