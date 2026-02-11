{
  description = "Local setup for Kuberpult development";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.systems.url = "github:nix-systems/default";
  inputs.flake-utils = {
    url = "github:numtide/flake-utils";
    inputs.systems.follows = "systems";
  };

  # nixpkgs revision that has libgit2 package with version 1.5.0
  inputs.nixpkgs-libgit2.url = "github:NixOS/nixpkgs/dc7ba75c10f017061ab164bab59e4b49fa6b2efe";

  outputs =
    { nixpkgs, flake-utils, nixpkgs-libgit2, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        old-pkgs = import nixpkgs-libgit2 {
          inherit system;
        };

        pkgs = import nixpkgs {
          inherit system;
        };
        packages = [
          # general build setup
          pkgs.gnumake

          # libgit
          old-pkgs.libgit2

          # docker
          pkgs.docker
          pkgs.docker-compose

          # go
          pkgs.go_1_25
          pkgs.golangci-lint

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
          buildInputs = packages;
          shellHook = ''
            if command -v zsh >/dev/null 2>&1 && [ -z "$DIRENV" ] && [ -z "$DIRENV_IN_ENVRC" ] && [ -z "$ZSH_VERSION" ]; then
              exec zsh
            fi
          '';
        };
      }
    );
}
