# Sourced into the Nix-built dots-herdr-plugins command. Arguments are
# source/revision pairs generated from config/herdr/plugins.toml.
if ! command -v herdr >/dev/null 2>&1; then
  echo 'herdr: install Herdr before syncing plugins; rerun dots apply.' >&2
  exit 1
fi
if ! command -v bun >/dev/null 2>&1; then
  echo 'herdr: Bun is required for Annotate; run proto use, then rerun dots apply.' >&2
  exit 1
fi

failed=0
while [ "$#" -ge 2 ]; do
  source=$1
  revision=$2
  shift 2
  if ! herdr plugin install "$source" --ref "$revision" --yes; then
    echo "herdr: failed to install $source at $revision" >&2
    failed=1
  fi
done
if [ "$#" -ne 0 ] || [ "$failed" -ne 0 ]; then
  exit 1
fi

herdr config check
if herdr status server >/dev/null 2>&1; then
  herdr server reload-config
fi
