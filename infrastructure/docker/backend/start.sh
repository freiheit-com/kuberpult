#!/bin/sh

cd /code/services/cd-service

export WITHOUT_DOCKER=true
make run
