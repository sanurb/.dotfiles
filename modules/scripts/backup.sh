#!/usr/bin/env bash
# Brownfield-safe backup of pre-existing dotfiles. Quarantines real
# files/dirs that would collide with home-manager-managed symlinks,
# leaving existing symlinks (prior home-manager activations) untouched.
# Invoked by `moon run dotfiles:backup`.

set -euo pipefail

PATHS=(
  ".config/ghostty/config"
  ".config/zellij/config.kdl"
  ".config/fish/config.fish"
  ".config/starship.toml"
  ".config/nvim/init.lua"
  ".zshrc"
  ".bashrc"
  ".bash_profile"
)

STAMP="$(date +%Y%m%d_%H%M%S)"
BACKUP_DIR="${HOME}/.dots_backups/${STAMP}"
COLLISIONS=()

for rel in "${PATHS[@]}"; do
  abs="${HOME}/${rel}"
  # Existing symlink → assume home-manager owns it; skip.
  # Existing real file/dir → collision.
  if [[ -e "$abs" && ! -L "$abs" ]]; then
    COLLISIONS+=("$rel")
  fi
done

if [[ ${#COLLISIONS[@]} -eq 0 ]]; then
  gum style --foreground 82 "No collisions detected. Safe to deploy."
  exit 0
fi

gum style --border rounded --padding "1 2" --foreground 212 \
  "Brownfield collisions found:"
printf '  • %s\n' "${COLLISIONS[@]}"

if ! gum confirm "Move these into ${BACKUP_DIR} and continue?"; then
  gum style --foreground 196 "Aborted by user. No changes made."
  exit 1
fi

mkdir -p "$BACKUP_DIR"
for rel in "${COLLISIONS[@]}"; do
  src="${HOME}/${rel}"
  dst="${BACKUP_DIR}/${rel}"
  mkdir -p "$(dirname "$dst")"
  mv "$src" "$dst"
done

gum style --foreground 82 "Backed up ${#COLLISIONS[@]} path(s) → ${BACKUP_DIR}"
