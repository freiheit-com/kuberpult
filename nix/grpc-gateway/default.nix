{ pkgs }: pkgs.buildGoApplication {
  pname = "protoc-gen-grpc-gateway";
  src = builtins.fetchGit {
    url = "https://github.com/grpc-ecosystem/grpc-gateway";
    rev = "f60dfa5c7b5fc2ce944f7625b551a63f8c4ea93e";
  };
  version = "v2.6.0";
  modules = ./gomod2nix.toml;
}
