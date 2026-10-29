# Tinycast

Optional macOS launcher preferences adapted from dmmulroy/.dotfiles. This repo
manages `settings.json` through `modules/home/foundation.nix` on macOS; it does
not install Tinycast, trust a Homebrew tap, or change macOS permissions.

Upstream distributes Tinycast through `abue-ammar/tinycast` as a self-signed
application. Review that source and its signing/trust requirements before
installing it separately.

## Configuration

1. After applying Home Manager, open Tinycast and enable **Settings → Backup →
   Settings File**. Choose **Import** when it detects the existing preferences,
   not Replace. The file alone does nothing until Settings File is enabled.
2. Review the Command-Space launcher shortcut, window-management shortcuts, and
   clipboard retention settings before enabling those features. Avoid running
   simultaneous clipboard/snippet managers or assigning Command-Space to
   multiple launchers.
3. Grant Accessibility yourself if you want clipboard paste, snippets, or window
   actions. Configure launch-at-login yourself.

Tinycast preserves the settings symlink. GUI changes rewrite the tracked JSON;
review diffs for personal paths or prompts before committing. Credentials,
history, databases, extensions, AI, and other third-party capabilities remain
machine-local and require separate setup.
