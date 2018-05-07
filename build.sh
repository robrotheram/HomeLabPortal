#!/bin/bash

mkdir -p build
rm -r build/*

cp -r config build/.
cp -r static build/.
cp -r templates build/.

go build -o build/HomeLabPortal
env GOOS=linux GOARCH=arm GOARM=5 go build -o build/HomeLabPortal-arm

cp build/HomeLabPortal* .

#tar -zcvf HomeLabPortal.tar.gz build
