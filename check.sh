#!/usr/bin/env bash

# ./check.sh
# Adds licence header to every relevant file.
# NOTE: This script assumes that in any file there is either no licence header or exactly the one listed here.

GO_COPY_RIGHT="/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the Expat(MIT) License as published by
the Free Software Foundation.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
MIT License for more details.

You should have received a copy of the MIT License
along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

Copyright freiheit.com*/"

YAML_COPY_RIGHT="# This file is part of kuberpult.

# Kuberpult is free software: you can redistribute it and/or modify
# it under the terms of the Expat(MIT) License as published by
# the Free Software Foundation.

# Kuberpult is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# MIT License for more details.

# You should have received a copy of the MIT License
# along with kuberpult. If not, see <https://directory.fsf.org/wiki/License:Expat>.

# Copyright freiheit.com"

RET_CODE=0
set eu -pipefail

check_file() {
    x=$(head -n 15 $1 | wc -l)
    if [ $x -lt 15 ];
    then
        return 1
    fi
    # check the first 15 lines
    lines=$(head -n 15 $1 )
    if [[ $2 -eq 1 ]];
    then
        if [ "$lines" = "$GO_COPY_RIGHT" ];
        then
            return 0
        fi
    else
        if [ "$lines" = "$YAML_COPY_RIGHT" ];
        then
            return 0
        fi
    fi
    return 1
}

# Read all go files
go_files=$(find . -type f -name *.go)


fix_file() {
    check_file $1 1
    if [ $? -ne 0 ];
    then
        echo "error in file $1"
        RET_CODE=1
        FILE=$(cat $1)
        cat > $1 <<- EOF
$GO_COPY_RIGHT
$FILE
EOF
    fi
}

fix_file_yaml_make() {
    check_file $1 2
    if [ $? -ne 0 ];
    then
        echo "error in file $1"
        RET_CODE=1
        FILE=$(cat $1)
        cat > $1 <<- EOF
$YAML_COPY_RIGHT
$FILE
EOF
    fi
}

echo fixing go files...
for go_file in $go_files
do
    fix_file "$go_file"
done

# Read all scss files
css_files=$(find . -type f -name '*.scss')

echo fixing css files...
for css_file in $css_files
do
    if [[ $css_file =~ .*node_modules.* ]];
    then
        continue
    fi
    fix_file "$css_file"
done

# Read all ts files
ts_files=$(find . -type f -name '*.ts')

echo fixing ts files...
for ts_file in $ts_files
do
    if [[ $ts_file =~ .*node_modules.* ]];
    then
        continue
    fi
    fix_file "$ts_file"
done

# Read all tsx files
tsx_files=$(find . -type f -name '*.tsx')
echo fixing tsx files...
for tsx_file in $tsx_files
do
    if [[ $tsx_file =~ .*node_modules.* ]];
    then
        continue
    fi
    fix_file "$tsx_file"
done

# Read all yaml files
yaml_files=$(find . -type f -name '*.yaml')
echo fixing yaml files...
for yaml_file in $yaml_files
do
    if [[ $yaml_file =~ .*pnpm-lock.* ]] || [[ $yaml_file =~ .*Buildfile.yaml.* ]];
      then
          continue
    fi
    fix_file_yaml_make $yaml_file
done

# Read all c/h files
c_files=$(find . -type f -name '*.[ch]')
echo fixing c files...
for c_file in $c_files
do
    fix_file "$c_file"
done


# Read all Make files
make_files=$(find . -type f -name "Makefile*")
echo fixing make files...
for make_file in $make_files
do
    if [[ $make_file =~ .*node_modules.* ]];
    then
        continue
    fi
    fix_file_yaml_make "$make_file"
done

exit $RET_CODE
