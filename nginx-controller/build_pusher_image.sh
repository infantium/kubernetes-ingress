#!/usr/bin/env bash
gcloud docker -- pull gcr.io/robbie-ai/gcloud-server-alpine36/latest:latest
make clean
make PREFIX=gcr.io/robbie-ai/nginx-ingress TAG=alpine36.v1 PUSH_TO_GCR=1