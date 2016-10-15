#!/usr/bin/env bash
make clean
make PREFIX=eu.gcr.io/infantium-platform-20/nginx-ingress TAG=latest PUSH_TO_GCR=1