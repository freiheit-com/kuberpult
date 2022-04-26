# kuberpult Readme for developers

## Release a new version

Building with libgit2 is tricky atm. Run `./dmake make -C services/cd-service bin/main` once to generate the binary for the cd-service.
Afterwards run `make release`. This will push the docker image, package the helm chart and create a git tag. The helm chart must be uploaded manually to the github release at the moment.
Afterwards bump the version in the `version` file.

## Install dev tools

- libgit2 >= 1.0

  Download tar file and follow instructions here: https://github.com/libgit2/libgit2#installation
  It worked for me to run: (the instructions are slightly different)
  ```
  sudo apt-get install libssl-dev
  mkdir build && cd build
  cmake ..
  sudo cmake --build . --target install
  ```
  Afterwards, set your library path, e.g.: `export LD_LIBRARY_PATH='/usr/local/lib/'`
- Chart Testing: 
  - install `helm`, `Yamale`, `Yamllint` as prerequisites to `ct` from https://github.com/helm/chart-testing#installation 
  - then follow the instructions to install `ct`
- golang >= 1.16
- protoc >=3.15
- buf from https://docs.buf.build/installation

There is a dev image based on alpine in `docker/build`. You can create a shell using the `./dmake` command.

## libgit2 vs. ...

The first version of this tool was written using go-git v5. Sadly the performance was abysmal. Adding a new manifest took > 20 seconds. Therefore, we switched to libgit2 which is much faster but less ergonomic.
