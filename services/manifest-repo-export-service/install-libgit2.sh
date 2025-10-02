#!/bin/bash
set -e 

# Clone and install libgit2 v1.5.0
git clone https://github.com/libgit2/libgit2.git
cd libgit2
git checkout v1.5.0

mkdir build
cd build
cmake .. -DCMAKE_INSTALL_PREFIX=/usr -DBUILD_SHARED_LIBS=ON -DUSE_SSH=ON -DUSE_REGEX=builtin
cmake --build . --target install

# Clean up
cd ../..
rm -rf libgit2
