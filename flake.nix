{
  description = "kuberpult";

  inputs.flake-utils.url = "github:numtide/flake-utils";

  inputs.nixpkgs.url = "github:nixos/nixpkgs";

  inputs.grpc-gateway = {
    url = "github:grpc-ecosystem/grpc-gateway";
    flake = false;
  };

  inputs.gomod2nix = {
    url = "github:tweag/gomod2nix";
    inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, flake-utils, grpc-gateway, gomod2nix }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        # General setup
        pkgs_ = nixpkgs.legacyPackages.${system};
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ gomod2nix.overlays.default ];
        };
        version = pkgs.readFile ./version;
        protoc-gen-grpc-gateway = pkgs.callPackage ./nix/grpc-gateway/default.nix { inherit pkgs; };
        protoc-gen-ts-proto = (pkgs.callPackage ./nix/ts-proto/default.nix { pkgs = pkgs_; nodejs = pkgs_.nodejs; }).ts-proto;

        # Native Build inputs
        nativeBuildInputs = [
          # go build
          pkgs.pkgconfig
          pkgs.gnumake
          pkgs.gomod2nix
          pkgs.go_1_19

          # nodejs build
          pkgs.nodePackages.pnpm
          pkgs.nodejs-slim
          pkgs.nodePackages.node2nix

          # chart build
          pkgs.kubernetes-helm
          pkgs.envsubst

          # build tools
          pkgs.jq

          # protobuf generation
          pkgs.buf
          pkgs.protoc-gen-go
          pkgs.protoc-gen-go-grpc
          protoc-gen-grpc-gateway
          protoc-gen-ts-proto
        ];

        # Target Build inputs
        buildInputs = [
          pkgs.libgit2_1_5
          pkgs.sqlite
        ];
      in
      rec {

        packages = {
          "protoc-gen-grpc-gateway" = protoc-gen-grpc-gateway;
        };

        # Creates a dev shell that has all dependencies preloaded
        devShell = pkgs.mkShell {
          inherit nativeBuildInputs buildInputs;
        };
      }
    );
}
