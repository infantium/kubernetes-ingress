#!/usr/bin/env bash
make clean
make PREFIX=gcr.io/robbie-ai/nginx-ingress TAG=latest PUSH_TO_GCR=1