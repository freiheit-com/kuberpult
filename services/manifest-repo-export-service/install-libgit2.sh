#!/bin/bash
set -e 

apk add --no-cache git

# Clone and install libgit2 v1.5.0
git clone https://github.com/libgit2/libgit2.git
cd libgit2
git checkout v1.5.0

# git comes with "pcre2" which currently has a vulnerability, so we remove it again
# git cannot be removed, as it is needed for the tests execution
#apk del git

mkdir build
cd build

# USE_REGEX=builtin stops libgit from using pcre:
cmake .. -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_SHARED_LIBS=ON -DUSE_SSH=ON -DUSE_REGEX=builtin
cmake --build . --target install

# Clean up
cd ../..
rm -rf libgit2
