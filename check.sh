#!/usr/bin/env bash

GO_COPY_RIGHT="/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/"

YAML_COPY_RIGHT="#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2021 freiheit.com"


check_file() {
    x=$(head -n 16 $1 | wc -l)
    if [ $x -lt 16 ];
    then
        return 1
    fi
    # check the first 16 lines
    lines=$(head -n 16 $1 )
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
        FILE=$(cat $1)
        cat > $1 <<- EOF
/*This file is part of kuberpult.

Kuberpult is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

Kuberpult is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

Copyright 2021 freiheit.com*/
$FILE
EOF
    fi
}

fix_file_yaml_make() {
    check_file $1 2
    if [ $? -ne 0 ];
    then
        FILE=$(cat $1)
        cat > $1 <<- EOF
#This file is part of kuberpult.

#Kuberpult is free software: you can redistribute it and/or modify
#it under the terms of the GNU General Public License as published by
#the Free Software Foundation, either version 3 of the License, or
#(at your option) any later version.

#Kuberpult is distributed in the hope that it will be useful,
#but WITHOUT ANY WARRANTY; without even the implied warranty of
#MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#GNU General Public License for more details.

#You should have received a copy of the GNU General Public License
#along with kuberpult.  If not, see <http://www.gnu.org/licenses/>.

#Copyright 2021 freiheit.com
$FILE
EOF
    fi
}

for go_file in $go_files
do
    fix_file $go_file
done

# Read all ts files
ts_files=$(find . -type f -name *.ts)

for ts_file in $ts_files
do
    if [[ $ts_file =~ .*node_modules.* ]];
    then
        continue
    fi
    fix_file $ts_file
done

# Read all tsx files
tsx_files=$(find . -type f -name *.tsx)

for tsx_file in $tsx_files
do
    if [[ $tsx_file =~ .*node_modules.* ]];
    then
        continue
    fi
    fix_file $tsx_file
done

# Read all yaml files
yaml_files=$(find . -type f -name *.yaml)

for yaml_file in $yaml_files
do
    fix_file_yaml_make $yaml_file
done

# Read all Make files
make_files=$(find . -type f -name Makefile*)

for make_file in $make_files
do
    fix_file_yaml_make $make_file
done
