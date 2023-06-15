{ pkgs }: pkgs.buildGoApplication {
  pname = "protoc-gen-grpc-gateway";
  src = builtins.fetchGit {
    url = "https://github.com/grpc-ecosystem/grpc-gateway";
    rev = "0eb17c3d70415c44406b0f02265ceaad570d6fa3";
  };
  version = "v2.15.2";
  modules = ./gomod2nix.toml;
}
