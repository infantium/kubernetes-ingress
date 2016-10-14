#!/usr/bin/env bash
make clean
make PREFIX=eu.gcr.io/infantium-platform-20/nginx-pusher-ingress TAG=0.1 PUSH_TO_GCR=1