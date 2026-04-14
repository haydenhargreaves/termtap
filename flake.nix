{
  description = "TermTap Development Flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { nixpkgs, flake-utils, ... }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
        {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            go-tools
            gcc_multi
            glibc_multi
          ];

          name = "TermTap";
          inherit (pkgs) zsh;

          shellHook = ''
            # Use the .local directory instead of home
            export GOPATH="$HOME/.local/go"
            echo "Settings GOPATH to: $HOME/.local/go "

            export GOOS=linux
            export GOARCH=amd64
            export CGO_CFLAGS=-Wno-error=cpp;

            exec zsh
          '';
        };
      }
    );
}
