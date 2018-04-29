#!/bin/sh
cd /HomeLabPortal
ls
nohup traefik -c traefik.toml > traefik.log 2>&1 & echo $! > run.pid
./HomeLabPortal
