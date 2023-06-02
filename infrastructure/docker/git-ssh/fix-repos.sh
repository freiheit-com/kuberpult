#!/bin/sh

PUSR=git
PHOME=/${PUSR}
PREPOS=${PHOME}/repos

# If Git repositories are present, fix permissions and set `SGID` bits
if [ -n "$(ls -A ${PREPOS}/)" ]; then
    chown -R ${PUSR}:${PUSR} ${PREPOS}/
    chmod -R ug+rwX ${PREPOS}/
    find ${PREPOS}/ -type d -exec chmod g+s '{}' +
fi
