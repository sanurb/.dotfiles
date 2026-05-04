# dots install — wizard end-to-end demo
#
# Render with:  vhs docs/demos/install.tape
# Output:       docs/demos/install.gif
#
# This tape drives the standalone path so it runs without Nix or a
# workspace clone — that keeps the demo deterministic in CI. The flow
# matches the screen primitive's slot order: stepper → header →
# description → option list with cursor → footer.

Output docs/demos/install.gif

Set FontSize    14
Set Width       960
Set Height      540
Set Theme       "Tokyo Night Storm"
Set Padding     20
Set Framerate   30

# Capture into a throwaway XDG_CONFIG_HOME so the demo never touches
# the host's real ~/.config/dots/.
Env XDG_CONFIG_HOME /tmp/dots-vhs-demo

Type "dots install"
Sleep 500ms
Enter
Sleep 1500ms

# Welcome
Enter
Sleep 1s

# Step 1 — Shell (cursor lands on the prior choice; pick zsh below)
Down
Sleep 600ms
Enter
Sleep 800ms

# Step 2 — Terminal (default Ghostty is fine)
Enter
Sleep 800ms

# Step 3 — Multiplexer (default Zellij)
Enter
Sleep 800ms

# Step 4 — Editor (Yes, install Neovim)
Enter
Sleep 800ms

# Step 5 — Git (Yes, apply defaults)
Enter
Sleep 800ms

# Step 6 — Confirm
Sleep 1s
Enter
Sleep 1500ms
