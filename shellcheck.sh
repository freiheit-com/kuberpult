#!/usr/bin/env bash

format=${1-quiet}
set -eu pipefail

RET_CODE=0

# Get all shell scripts
shell_scripts=$(find . -type f -name "*.sh")

for shell_script in $shell_scripts
do
    if [[ $shell_script =~ .*node_modules.* ]];
    then
        continue
    fi

    if [[ $shell_script =~ .*\.git.* ]];
    then
        continue
    fi

    if ! shellcheck -f "$format" "$shell_script"
    then
        RET_CODE=1
        if [ "$format" = "quiet" ]; then
            echo "error in file $shell_script"
        fi
    fi
done

exit $RET_CODE
