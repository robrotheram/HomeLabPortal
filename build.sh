#!/bin/bash

mkdir -p build
rm -r build/*

cp -r config build/.
cp -r static build/.
cp -r templates build/.
cp Dockerfile build/.
cp traefik.toml build/.
cp entrypoint.sh build/.

CGO_ENABLED=0 GOOS=linux go build -o build/HomeLabPortal -a -ldflags '-extldflags "-static"' .
env GOOS=linux GOARCH=arm GOARM=5 go build -o build/HomeLabPortal-arm
cd build
tar -zcvf ../HomeLabPortal.tar.gz *
