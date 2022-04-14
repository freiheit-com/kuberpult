#!/bin/sh

cd /code
make .install

cd /code/services/cd-service

export WITHOUT_DOCKER=true
make run
