{ pkgs }:
let
  # dprint self-traverses rather than consuming treefmt's file list, so both
  # layers must receive the same exclusions. Define them once to prevent a
  # generated or vendored path from being protected at only one layer.
  formatterExcludes = [
    "config/nvim/lazy-lock.json"
    "config/nvim/.undodir/**"
    "config/opencode/package-lock.json"
    "config/pi/package-lock.json"
    "config/pi/node_modules/**"
    "config/pi/agent/extensions/**/node_modules/**"
    # Vendored skill content (mattpocock/skills + Cloudflare docs bundle) —
    # reformatting would churn every future re-vendor diff.
    "config/pi/agent/skills/**"
    "flake.lock"
    "*.lock"
    ".moon/cache/**"
    ".devenv/**"
    ".direnv/**"
  ];
in
{
  projectRootFile = "flake.nix";

  # Tool-managed and machine-generated files are off-limits to every
  # formatter — reformatting them either fights another writer
  # (lazy.nvim, npm, Nix) or adds churn that obscures real diffs.
  settings.global.excludes = formatterExcludes;

  programs.gofumpt.enable = true;

  # nixfmt is the RFC-166 official Nix formatter; nixpkgs-fmt is archived.
  # nixpkgs ≥ 25.11 ships nixfmt as the default and the `nixfmt-rfc-style`
  # alias is a deprecation shim.
  programs.nixfmt.enable = true;

  # dprint covers the Markdown / JSON / TOML / YAML surface that native Go
  # and Nix formatters don't touch. Wasm plugins are pinned via nixpkgs so
  # formatting performs no network fetches.
  programs.dprint = {
    enable = true;
    includes = [
      "*.md"
      "*.json"
      "*.jsonc"
      "*.toml"
      "*.yaml"
      "*.yml"
    ];
    settings = {
      lineWidth = 100;
      indentWidth = 2;
      excludes = formatterExcludes;
      markdown.textWrap = "maintain";
      json = { };
      toml = { };
      yaml = { };
      plugins = pkgs.dprint-plugins.getPluginList (
        plugins: with plugins; [
          dprint-plugin-markdown
          dprint-plugin-json
          dprint-plugin-toml
          g-plane-pretty_yaml
        ]
      );
    };
  };
}
