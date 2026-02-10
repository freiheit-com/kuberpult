{
  description = "Local setup for Kuberpult development";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  inputs.systems.url = "github:nix-systems/default";
  inputs.flake-utils = {
    url = "github:numtide/flake-utils";
    inputs.systems.follows = "systems";
  };

  outputs =
    { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs {
          inherit system;
        };
        packages = [
          # general build setup
          pkgs.gnumake

          # libgit
          pkgs.libgit2

          # docker
          pkgs.docker
          pkgs.docker-compose

          # go
          pkgs.go_1_24
          pkgs.golangci-lint

          # go plugins
          pkgs.protobuf_30
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          pkgs.oapi-codegen

          # rpgc utilities
          pkgs.evans
          pkgs.grpcurl

          # frontend
          pkgs.nodejs_24
          pkgs.pnpm
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
