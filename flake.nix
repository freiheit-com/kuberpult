{
  description = "kuberpult";

  inputs.flake-utils.url = "github:numtide/flake-utils";

  inputs.nixpkgs.url = "github:nixos/nixpkgs";

  inputs.grpc-gateway = {
    url   = "github:grpc-ecosystem/grpc-gateway";
    flake = false;
  };

  inputs.gomod2nix = {
    url   = "github:tweag/gomod2nix";
    inputs.nixpkgs.follows = "nixpkgs";
  };

  outputs = { self, nixpkgs, flake-utils, grpc-gateway, gomod2nix }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        # General setup
        pkgs_ = nixpkgs.legacyPackages.${system};
	pkgs = import nixpkgs {
		inherit system;
		overlays = [ gomod2nix.overlay ];
	};
        protoc-gen-grpc-gateway = pkgs.callPackage ./nix/grpc-gateway/default.nix { inherit pkgs; };

	# Native Build inputs
        nativeBuildInputs = [
	    # go build
            pkgs.go
	    pkgs.pkgconfig
	    pkgs.gnumake
	    pkgs.gomod2nix
	    
	    # protobuf generation
	    pkgs.buf
	    pkgs.protoc-gen-go
	    pkgs.protoc-gen-go-grpc
	    protoc-gen-grpc-gateway

	    # nodejs build
	    pkgs.yarn
	    pkgs.yarn2nix
	    pkgs.nodejs-slim

	    # chart build
	    pkgs.kubernetes-helm
	    pkgs.envsubst

	    # build tools
	    pkgs.jq
	];

	# Target Build inputs
	buildInputs = [
            pkgs.libgit2
	];

	# Default source
	src  = pkgs.nix-gitignore.gitignoreSource [] ./.;

	# cd-service
	cd-service = pkgs.stdenv.mkDerivation {
	  name = "cd-service";
	  inherit src buildInputs nativeBuildInputs;
	  buildPhase = ''
	    export GOPATH=$TMPDIR/gopath
	    export GOCACHE=$TMPDIR/gocache
	    export XDG_CACHE_HOME=$TMPDIR/.cache
	    make -C services/cd-service bin/main 
	  '';
	  installPhase = ''
            mkdir -p $out/bin
            cp ./services/cd-service/bin/main $out/bin/
	  '';
	};

	# frontend-service
	frontend-service = pkgs.stdenv.mkDerivation {
	  name = "frontend-service";
	  inherit src buildInputs nativeBuildInputs;
	  buildPhase = ''
	    export GOPATH=$TMPDIR/gopath
	    export GOCACHE=$TMPDIR/gocache
	    export XDG_CACHE_HOME=$TMPDIR/.cache
	    export YARN_CACHE_FOLDER=$TMPDIR/yarn
	    echo "disable-self-update-check true" > $TMPDIR/.yarnrc
	    export YARN_OPTS=--use-yarnrc=$TMPDIR/.yarnrc
	    make -C services/frontend-service build
	  '';
	  installPhase = ''
            mkdir -p $out/bin
            make -C ./services/frontend-service install DESTDIR=$out
	  '';
	};

	version = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./version);
      in rec {
        packages = {
	  "protoc-gen-grpc-gateway" = protoc-gen-grpc-gateway;
	  "services/cd-service" = cd-service;
	  "services/cd-service/docker" = pkgs.dockerTools.streamLayeredImage {
            name = "cd-service";
	    tag  = version;
	    contents = [ self.packages.x86_64-linux."services/cd-service" pkgs.tzdata ];
	  };
          "services/frontend-service" = frontend-service;
	  "services/frontend-service/docker" = pkgs.dockerTools.streamLayeredImage {
            name = "frontend-service";
	    tag  = version;
	    contents = [ self.packages.x86_64-linux."services/frontend-service" pkgs.tzdata ];
	  };
        };
	apps = {
	  "services/cd-service/docker" = {
	    type = "app";
	    program = "${self.packages.${system}."services/cd-service/docker"}";
	  };
          "services/frontend-service/docker" = {
	    type = "app";
	    program = "${self.packages.${system}."services/frontend-service/docker"}";
	  };
	};
	# Creates a dev shell that has all dependencies preloaded
	devShell = pkgs.mkShell {
           inherit nativeBuildInputs buildInputs;
	};
      }
   );
}
