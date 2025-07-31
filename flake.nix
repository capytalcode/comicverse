{
  description = "My development environment";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };
  outputs = {nixpkgs, ...}: let
    systems = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];
    forAllSystems = f:
      nixpkgs.lib.genAttrs systems (system: let
        pkgs = import nixpkgs {inherit system;};
      in
        f system pkgs);
  in {
    devShells = forAllSystems (system: pkgs: {
      default = pkgs.mkShell {
        CGO_ENABLED = "1";
        hardeningDisable = ["fortify"];

        shellHook = ''
          set -a
          source .env
          set +a
        '';

        buildInputs = with pkgs; [
          # Go tools
          go
          golangci-lint
          gofumpt
          gotools
          delve

          # TailwindCSS
          tailwindcss_4

          # Sqlite tools
          sqlite
          lazysql
          litecli

          # S3
          awscli

          # ePUB
          http-server
          calibre
          zip
          unzip
        ];
      };
    });
  };
}
