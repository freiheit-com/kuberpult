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
	protoc-gen-ts-proto = (pkgs.callPackage ./nix/ts-proto/default.nix { pkgs = pkgs_; nodejs = pkgs_.nodejs; } ).ts-proto;

	# Native Build inputs
        nativeBuildInputs = [
	    # go build
	    pkgs.pkgconfig
	    pkgs.gnumake
	    pkgs.gomod2nix
	    pkgs.go_1_17
	    
	    # nodejs build
	    pkgs.yarn
	    pkgs.yarn2nix
	    pkgs.nodejs-slim
	    pkgs.nodePackages.node2nix

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
	cd-service = pkgs.buildGoApplication {
	  name = "cd-service";
	  modules = ./gomod2nix.toml;
	  inherit src buildInputs nativeBuildInputs;
	  buildPhase = ''
	    make -C services/cd-service bin/main 
	  '';
	  installPhase = ''
            mkdir -p $out/bin
            cp ./services/cd-service/bin/main $out/bin/
	  '';
	};

	# frontend-service
	frontend-service = pkgs.buildGoApplication {
	  name = "frontend-service";
	  modules = ./gomod2nix.toml;
	  inherit src buildInputs nativeBuildInputs;
	  buildPhase = ''
	    export GOPATH=$TMPDIR/gopath
	    export GOCACHE=$TMPDIR/gocache
	    export XDG_CACHE_HOME=$TMPDIR/.cache
	    make -C services/frontend-service build
	  '';
	  installPhase = ''
            mkdir -p $out/bin
            make -C ./services/frontend-service install DESTDIR=$out
	  '';
	};

	# protos
	protos = pkgs.stdenv.mkDerivation {
	  name = "protos";
	  inherit src;

	  buildInputs = [
	    # protobuf generation
	    pkgs.buf
	    pkgs.protoc-gen-go
	    pkgs.protoc-gen-go-grpc
	    protoc-gen-grpc-gateway
	    protoc-gen-ts-proto
	  ];

	  buildPhase = ''
	    mkdir $TMPDIR/out
	    buf generate --output $TMPDIR/out
	  '';
	  installPhase = ''
	    cp -r $TMPDIR/out $out
	  '';
	};
	update-protos = pkgs.writeShellApplication {
	  name = "update-protos";

	  runtimeInputs = [
	    # protobuf generation
	    pkgs.buf
	    pkgs.protoc-gen-go
	    pkgs.protoc-gen-go-grpc
	    protoc-gen-grpc-gateway
	    protoc-gen-ts-proto
	  ];

	  text = "buf generate";
	};

	# frontend-service
	ui = pkgs.mkYarnPackage {
	  name = "ui";
	  src  = pkgs.nix-gitignore.gitignoreSource [] ./services/frontend-service;
	  inherit buildInputs nativeBuildInputs;

    buildPhase = ''
    cd deps/kuberpult
    cp -r ${protos}/services/frontend-service/src/api/ src/api/
    export CACHE_DIR=$TMPDIR
    yarn --offline build
    '';
    preDist = ''
    export CACHE_DIR=$TMPDIR
    '';
	};
 
        in rec {

        packages = {
	  "protoc-gen-grpc-gateway" = protoc-gen-grpc-gateway;
	  "services/cd-service" = cd-service;
	  "services/cd-service:docker" = pkgs.dockerTools.streamLayeredImage {
            name = "cd-service";
	    contents = [ self.packages.x86_64-linux."services/cd-service" pkgs.tzdata ];
	  };
          "services/frontend-service" = frontend-service;
	  "services/frontend-service:docker" = pkgs.dockerTools.streamLayeredImage {
            name = "frontend-service";
	    contents = [ self.packages.x86_64-linux."services/frontend-service" pkgs.tzdata ];
	  };
	  "ui" = ui;
	  "protos" = protos;
	  "update-protos" = update-protos;
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
	  "buf" = {
            type = "app";
	    program = "${update-protos}/bin/update-protos";
	  };
	};
	# Creates a dev shell that has all dependencies preloaded
	devShell = pkgs.mkShell {
           inherit nativeBuildInputs buildInputs;
	};
      }
   );
}
