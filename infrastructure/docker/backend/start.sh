#!/bin/sh

exec make -C /kp/kuberpult/services/cd-service run WITHOUT_DOCKER=true
