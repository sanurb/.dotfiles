{ ... }:
{
  # tealdeer — fast Rust client for the tldr community cheatsheets
  # (binary: `tldr`). Complements navi (modules/home/navi.nix) without
  # overlapping it: navi is interactive, curated, and you author the
  # cheats yourself (plus a Ctrl+G shell widget); tldr is the canonical,
  # crowd-sourced "what's the common usage of $cmd" lookup. Reach for tldr
  # to recall a flag, navi to drive your own snippets.
  #
  # We use the `programs.tealdeer` HM module rather than a bare package
  # because it declaratively owns ~/.config/tealdeer/config.toml — and
  # that path is plain XDG, NOT one of our workspace-level
  # mkOutOfStoreSymlink targets, so the lazygit/fzf-fish collision concern
  # does not apply here. The module is the right tool precisely because
  # there is real config to set: we enable weekly cache auto-update so the
  # offline pages refresh on use instead of requiring a manual
  # `tldr --update`.
  programs.tealdeer = {
    enable = true;
    settings = {
      updates = {
        auto_update = true;
        auto_update_interval_hours = 168; # weekly
      };
    };
  };
}
