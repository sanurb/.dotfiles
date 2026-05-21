-- fff.nvim — file search backed by a Rust native lib (libfff_nvim).
--
-- modules/home/fff.nix builds the cdylib and copies the upstream
-- lua/, plugin/, doc/ tree under $out/share/fff.nvim, exposed to
-- this spec via vim.env.FFF_NVIM_DIR. When set, lazy.nvim uses
-- that store path as `dir` and skips both the GitHub clone and
-- the build-hook download. When unset (host where the fff module
-- is disabled, or DOTS_WORKSPACE_ROOT not exported), the spec
-- falls back to lazy.nvim's normal install path so the plugin
-- still works.

local fff_dir = vim.env.FFF_NVIM_DIR
local nix_managed = fff_dir ~= nil and fff_dir ~= ""

return {
	{
		"dmtrKovalenko/fff.nvim",
		dir = nix_managed and fff_dir or nil,
		build = nix_managed and false or function()
			require("fff.download").download_or_build_binary()
		end,
		lazy = false, -- the plugin lazy-initialises itself
		keys = {
			{ "<leader>ff", function() require("fff").find_files() end, desc = "FFFind files" },
			{ "<leader>fg", function() require("fff").live_grep() end, desc = "LiFFFe grep" },
			{ "<leader>fz",
				function() require("fff").live_grep({ grep = { modes = { "fuzzy", "plain" } } }) end,
				desc = "Live fffuzy grep",
			},
			{ "<leader>fc",
				function() require("fff").live_grep({ query = vim.fn.expand("<cword>") }) end,
				desc = "Search current word",
			},
		},
		opts = {},
	},
}
