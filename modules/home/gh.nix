{ ... }:
{
  # gh — GitHub's official CLI. ssh as the protocol so `gh repo clone`
  # produces SSH remotes that play nice with the SSH-format commit
  # signing wired up in modules/home/git.nix. Editor / pager track the
  # rest of the workspace (nvim, delta) so `gh pr view --web=false`
  # and `gh pr diff` look the same as the equivalent git commands.
  programs.gh = {
    enable = true;
    settings = {
      git_protocol = "ssh";
      prompt = "enabled";
      editor = "nvim";
      # delta needs --paging=never because gh wraps the pager itself;
      # without it the pager gets re-entered and arrow keys stop
      # working.
      pager = "delta --dark --side-by-side --paging=never";

      aliases = {
        co = "pr checkout";
        pv = "pr view";
        pc = "pr create";
        pl = "pr list";
        pm = "pr merge";
        prd = "pr diff";
        rv = "repo view";
        rc = "repo clone";
        il = "issue list";
        iv = "issue view";
        ic = "issue create";
      };
    };
  };

  # gh-dash — TUI dashboard. Default sections scoped to @me; host
  # overlays can extend.
  programs.gh-dash = {
    enable = true;
    settings = {
      prSections = [
        {
          title = "My PRs";
          filters = "is:open author:@me";
        }
        {
          title = "Review Requested";
          filters = "is:open review-requested:@me";
        }
        {
          title = "Involved";
          filters = "is:open involves:@me -author:@me";
        }
      ];
      issuesSections = [
        {
          title = "Assigned";
          filters = "is:open assignee:@me";
        }
        {
          title = "Created";
          filters = "is:open author:@me";
        }
      ];
      defaults = {
        preview = {
          open = true;
          width = 50;
        };
        prsLimit = 20;
        issuesLimit = 20;
      };
    };
  };
}
