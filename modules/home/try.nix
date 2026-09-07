{
  config,
  pkgs,
  lib,
  ...
}:
let
  # try isn't in nixpkgs. Upstream ships its own flake with a package and
  # a home module, but taking it as an input would add a second nixpkgs
  # evaluation to every `dots apply` for one 200-line Ruby script. We
  # vendor the derivation instead, same call as modules/home/amoxide.nix.
  #
  # The gemspec declares no runtime dependencies — try is stdlib-only
  # Ruby — so there is no gem closure to resolve and the build is a copy
  # plus an interpreter substitution. required_ruby_version is
  # ">= 3.0.0"; pkgs.ruby satisfies that and tracks nixpkgs rather than
  # pinning a minor upstream never asked for.
  try = pkgs.stdenvNoCC.mkDerivation (finalAttrs: {
    pname = "try";
    version = "1.10.1";

    src = pkgs.fetchFromGitHub {
      owner = "tobi";
      repo = "try";
      tag = "v${finalAttrs.version}";
      hash = "sha256-ZSt6LSp0AQTbdN86lJGJPWcx6oFR63AFi4s8Vjr5a5o=";
    };

    # The repo ships a Makefile whose default target prints help and
    # exits 1, so stdenv's default build phase fails. There is nothing to
    # compile here anyway — installPhase is a copy.
    dontBuild = true;

    # lib/ has to sit beside the entrypoint: try.rb resolves its requires
    # relative to __dir__, so installing the script alone yields a binary
    # that dies on its first require.
    #
    # Every `/usr/bin/env ruby` is rewritten to the store ruby, and that
    # is the whole point of not using a wrapper here. v1.10.1 hardcodes
    # that string into the shell functions `try init` emits, so wrapping
    # the binary fixes only direct invocation: the function still went
    # through the user's PATH, which on macOS is ruby 2.6.10 while try
    # requires >= 3.0. The result was `try` the binary working while `try`
    # the shell function — the actual interface, since it has to cd the
    # parent — died with `undefined method 'define' for Data:Class`.
    #
    # Three sites are covered by the one substitution: the shebang and
    # both emitted snippets (fish and POSIX). --replace-fail means a
    # version that stops spelling it this way fails the build instead of
    # silently reintroducing the PATH lookup.
    installPhase = ''
      runHook preInstall
      mkdir -p $out/bin
      cp try.rb $out/bin/try
      cp -r lib $out/bin/
      chmod +x $out/bin/try
      substituteInPlace $out/bin/try \
        --replace-fail '/usr/bin/env ruby' '${lib.getExe pkgs.ruby}'
      runHook postInstall
    '';

    doInstallCheck = true;
    installCheckPhase = ''
      runHook preInstallCheck
      # HOME and TRY_PATH are pinned into the build dir because try
      # mkdir -p's its tries directory on startup; with the sandbox's
      # read-only /homeless-shelter that aborts before it can print.
      # `--version` is the flag that reports; a bare `version` argument
      # falls through to the interactive selector.
      export HOME="$TMPDIR/home"
      export TRY_PATH="$TMPDIR/tries"
      # 2>&1 is required, not defensive: try prints its version banner on
      # stderr and leaves stdout empty, so piping stdout into grep matches
      # nothing and fails the build while try itself exits 0.
      reported=$("$out/bin/try" --version 2>&1)
      case "$reported" in
        *'${finalAttrs.version}'*) ;;
        *)
          echo "try --version reported '$reported', expected ${finalAttrs.version}" >&2
          exit 1
          ;;
      esac
      # The emitted shell function must be self-contained: any `env ruby`
      # in it means the user's PATH decides which interpreter runs try,
      # which is exactly the regression this package was fixed for.
      snippet=$(SHELL=/bin/bash "$out/bin/try" init 2>&1)
      case "$snippet" in
        *"env ruby"*)
          echo "try init emitted an interpreter lookup instead of a store path:" >&2
          echo "$snippet" >&2
          exit 1
          ;;
      esac

      runHook postInstallCheck
    '';

    meta = {
      description = "Dated directories for throwaway experiments, with fuzzy search";
      homepage = "https://github.com/tobi/try";
      license = lib.licenses.mit;
      platforms = lib.platforms.unix;
      mainProgram = "try";
    };
  });

  triesPath = "${config.home.homeDirectory}/src/tries";
in
{
  home.packages = [ try ];

  # try.rb reads TRY_PATH for the directory it creates tries under,
  # falling back to ~/src/tries. Declaring it makes the location explicit
  # and lets the init snippets below stay argument-free.
  home.sessionVariables.TRY_PATH = triesPath;

  # `try` has to be a shell *function*: the binary prints a cd command on
  # stdout and the function evals it, which a subprocess could never do
  # to its parent. `try init` emits that function.
  #
  # It picks the fish snippet over the POSIX one by inspecting $SHELL, not
  # by taking a shell argument, so a fish session started from another
  # shell would otherwise be handed bash syntax and fail to parse. Forcing
  # SHELL for the invocation makes the choice deterministic instead of
  # dependent on how the session was launched. Fish has no VAR=val command
  # prefix, hence `env`.
  programs.fish.interactiveShellInit = lib.mkAfter ''
    if command -q try
        env SHELL=fish try init | source
    end
  '';

  programs.zsh.initContent = lib.mkAfter ''
    if command -v try >/dev/null 2>&1; then
      eval "$(try init)"
    fi
  '';
}
