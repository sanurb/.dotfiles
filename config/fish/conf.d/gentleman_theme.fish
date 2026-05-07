# Fish syntax-highlighting palette. Hex codes mirror
# config/ghostty/themes/gentleman so prompt and terminal share one
# visual identity.

set -l foreground f3f6f9
set -l selection  263356
set -l comment    8a8fa3
set -l red        cb7c94
set -l orange     deba87
set -l yellow     ffe066
set -l green      b7cc85
set -l purple     a3b5d6
set -l cyan       7aa89f
set -l pink       ff8dd7

# Syntax highlighting
set -g fish_color_normal         $foreground
set -g fish_color_command        $cyan
set -g fish_color_keyword        $pink
set -g fish_color_quote          $yellow
set -g fish_color_redirection    $foreground
set -g fish_color_end            $orange
set -g fish_color_error          $red
set -g fish_color_param          $purple
set -g fish_color_comment        $comment
set -g fish_color_selection      --background=$selection
set -g fish_color_search_match   --background=$selection
set -g fish_color_operator       $green
set -g fish_color_escape         $pink
set -g fish_color_autosuggestion $comment

# Completion pager
set -g fish_pager_color_progress    $comment
set -g fish_pager_color_prefix      $cyan
set -g fish_pager_color_completion  $foreground
set -g fish_pager_color_description $comment
