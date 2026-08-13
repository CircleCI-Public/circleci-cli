# Copyright (c) 2026 Circle Internet Services, Inc.
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in
# all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.
#
# SPDX-License-Identifier: MIT
{
  description = "CircleCI CLI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        version = self.shortRev or self.dirtyShortRev or "dev";

        circleci-cli = pkgs.buildGoModule {
          pname = "circleci-cli";
          inherit version;

          src = self;

          # Update this hash after any go.mod / go.sum change by running:
          #   nix build --no-link 2>&1 | grep 'got:'
          # and pasting the hash shown there.
          vendorHash = "sha256-v6mXdAORGlbjtLezkoHIXK52h3nLeahihoc3OjF6JHA=";

          subPackages = [ "cmd/circleci" ];

          nativeBuildInputs = [ pkgs.installShellFiles ];

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];

          doCheck = false;

          postInstall = ''
            installShellCompletion --cmd circleci \
              --bash <(HOME=$TMPDIR $out/bin/circleci completion bash) \
              --fish <(HOME=$TMPDIR $out/bin/circleci completion fish) \
              --zsh <(HOME=$TMPDIR $out/bin/circleci completion zsh)
          '';

          meta = with pkgs.lib; {
            description = "Use and manage CircleCI from the command line";
            homepage = "https://circleci.com/docs/local-cli/";
            license = licenses.mit;
            mainProgram = "circleci";
          };
        };
      in
      {
        packages = {
          inherit circleci-cli;
          default = circleci-cli;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            go-task
          ];
        };
      }
    );
}
