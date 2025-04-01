{
  description = "IncusScripts CLI";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
  }: let
    supportedSystems = [
      "aarch64-linux"
      "x86_64-linux"
    ];
    forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);
  in {
    packages = forAllSystems (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};
      in {
        scripts-cli = pkgs.callPackage ./nix/package.nix {};
        default = self.packages.${system}.scripts-cli;
      }
    );
  };
}
