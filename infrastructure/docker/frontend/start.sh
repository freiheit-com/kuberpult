#!/bin/sh

cd /code
make .install

cd /code/services/frontend-service

make run
