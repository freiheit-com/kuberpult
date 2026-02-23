{
  description = "Local setup for Kuberpult development";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.systems.url = "github:nix-systems/default";
  inputs.flake-utils = {
    url = "github:numtide/flake-utils";
    inputs.systems.follows = "systems";
  };

  # libgit2 1.5.0
  inputs.nixpkgs-libgit2.url = "github:NixOS/nixpkgs/dc7ba75c10f017061ab164bab59e4b49fa6b2efe";
  inputs.nixpkgs-sqlite.url = "github:NixOS/nixpkgs/2343bbb58f99267223bc2aac4fc9ea301a155a16";
  inputs.nixpkgs-zlib.url = "github:NixOS/nixpkgs/8482c7ded03bae7550f3d69884f1e611e3bd19e8";
  inputs.nixpkgs-openssl.url = "github:NixOS/nixpkgs/a672be65651c80d3f592a89b3945466584a22069";

  # golangci-lint 2.9.0
  inputs.nixpkgs-golangci-lint.url = "github:NixOS/nixpkgs/5658e3793ef17f837c67f830a9d3bef3e12ecded";

  outputs =
    { nixpkgs, flake-utils, nixpkgs-libgit2, nixpkgs-golangci-lint, nixpkgs-sqlite, nixpkgs-zlib, nixpkgs-openssl, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        libgit-pkgs = import nixpkgs-libgit2 {
          inherit system;
        };

        golangci-pkgs = import nixpkgs-golangci-lint {
          inherit system;
        };
        pinned-sqlite = nixpkgs-sqlite.legacyPackages.${system}.sqlite;
        pinned-zlib = nixpkgs-zlib.legacyPackages.${system}.zlib;
        pinned-openssl = nixpkgs-openssl.legacyPackages.${system}.openssl;

        pkgs = import nixpkgs {
          inherit system;
        };
        packages = [
          # general build setup
          pkgs.gnumake

          # libgit
          libgit-pkgs.libgit2
          pinned-sqlite  # needed by libgit
          pinned-openssl # needed by libgit
          pinned-zlib    # needed by libgit

          # docker
          pkgs.docker
          pkgs.docker-compose

          # go
          pkgs.go_1_25
          golangci-pkgs.golangci-lint

          # go plugins
          pkgs.protobuf_30
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          pkgs.oapi-codegen

          # grpc utilities
          pkgs.evans
          pkgs.grpcurl

          # frontend
          pkgs.nodejs_24
          pkgs.pnpm_8
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          nativeBuildInputs = [pkgs.pkg-config];
          hardeningDisable = [ "fortify" ];
          buildInputs = packages;
          shellHook = ''
# These paths are important for running go tests in an IDE,
# otherwise it won't find opennssl/etc which is a requirement of libgit2:
export PKG_CONFIG_PATH="${pkgs.lib.makeSearchPathOutput "dev" "lib/pkgconfig" packages}"

# We do not want to overwrite the global LD_LIBRARY_PATH, as it can mess with git (e.g. when creating a signed commit)
# To make it work in the IDE, start it like this:
# LD_LIBRARY_PATH="$NIX_LD_LIBRARY_PATH" myIdeExecutable
export NIX_LD_LIBRARY_PATH="${pkgs.lib.makeLibraryPath packages}:$LD_LIBRARY_PATH"
            '';
        };
      }
    );
}
