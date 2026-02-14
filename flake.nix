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

  # golangci-lint 2.7.2
  inputs.nixpkgs-golangci-lint.url = "github:NixOS/nixpkgs/07c10b3282ff0a5b0fa3a684d5174c17e823929a";

  outputs =
    { nixpkgs, flake-utils, nixpkgs-libgit2, nixpkgs-golangci-lint, ... }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        libgit-pkgs = import nixpkgs-libgit2 {
          inherit system;
        };

        golangci-pkgs = import nixpkgs-golangci-lint {
          inherit system;
        };

        pkgs = import nixpkgs {
          inherit system;
        };
        packages = [
          # general build setup
          pkgs.gnumake

          # libgit
          libgit-pkgs.libgit2

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
        };
      }
    );
}
