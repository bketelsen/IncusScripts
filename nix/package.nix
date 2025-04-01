{
  lib,
  buildGo123Module,
  installShellFiles,
}:
buildGo123Module rec {
  pname = "scripts-cli";
  version = "latest";

  src = ./../cli;

  vendorHash = "sha256-iPxabKi33tJH8xQ9gPr9XmT0LcBwxtonnXJlJMnYxjw=";
  subPackages = ["." "./cmd"];

  nativeBuildInputs = [
    installShellFiles
  ];

  postInstall = ''
    # Rename the binary from cli to scripts-cli
    mv $out/bin/cmd $out/bin/scripts-cli

    installShellCompletion --cmd scripts-cli \
          --bash <($out/bin/scripts-cli completion bash) \
          --fish <($out/bin/scripts-cli completion fish) \
          --zsh <($out/bin/scripts-cli completion zsh)
  '';

  env.CGO_ENABLED = 0;

  ldflags = [
    "-X github.com/bketelsen/IncusScripts/cli/cmd/main.commit=${version}"
  ];

  meta = {
    description = "Incus Helper-Scripts";
    homepage = "https://github.com/bketelsen/IncusScripts";
    license = lib.licenses.mit;
    mainProgram = "scripts-cli";
    platforms = lib.platforms.linux;
  };
}
