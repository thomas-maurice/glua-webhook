{
  description = "glua-webhook development environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            kubectl
            kind
            curl
            wget
            gnumake
            golangci-lint
          ];

          shellHook = ''
            echo "glua-webhook development environment"
            echo "kubectl version: $(kubectl version --client --short 2>/dev/null || echo 'not found')"
            echo "kind version: $(kind version 2>/dev/null || echo 'not found')"
            echo "go version: $(go version)"
          '';
        };
      }
    );
}
