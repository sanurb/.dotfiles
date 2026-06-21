_: {
  # navi — interactive cheatsheet TUI. The `navi` binary works from any
  # shell (it prints the selected command on stdout). The widget hook
  # — Ctrl+G fuzzy-search that writes the chosen command into the
  # current command line for editing — is what HM wires in here.
  #
  # We use the `programs.navi` HM module so the widget lands in the
  # right rc file declaratively. Upstream supports bash / zsh / fish
  # widgets; the per-shell `enable<Bash|Zsh|Fish>Integration` options
  # default to the matching `programs.<shell>.enable` via
  # `lib.hm.shell.mkXIntegrationOption`. Because foundation.nix always
  # enables bash and zsh (and shells/fish.nix enables fish on the fish
  # persona), the widget is reachable from those three shells on every
  # host.
  #
  # Nushell gap: upstream navi has no nushell widget — `navi widget
  # nu` is not implemented. On a nushell-pillar host, the binary still
  # works from the command line, but Ctrl+G inside nushell does
  # nothing. Users who want the widget can drop into bash/zsh (both
  # always-enabled by foundation) and invoke it there, or run `navi`
  # directly and copy the output. We deliberately keep navi installed
  # on nushell hosts rather than gating it out — the CLI is still
  # useful and the trade-off is documented here.
  #
  # Cheats path stays at navi's default (~/.local/share/navi/cheats).
  # We don't pre-declare `programs.navi.settings` because an empty
  # cheats path list there would override navi's bundled defaults.
  programs.navi.enable = true;
}
