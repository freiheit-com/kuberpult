#!/bin/sh

PUSR=git

# Expected exit code
EXIT_CODE_EXP=128

su -l ${PUSR} -s /bin/sh -c \
    "ssh -q \
    -o \"UserKnownHostsFile=/dev/null\" \
    -o \"StrictHostKeyChecking=no\" \
    ${PUSR}@localhost > /dev/null 2>&1"

if [ "$?" -eq "${EXIT_CODE_EXP}" ]; then
    exit 0 # Pass
else
    exit 1 # Fail
fi
