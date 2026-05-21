-- FFF_NVIM_DIR points at modules/home/fff.nix's $out/share/fff.nvim.
-- The cdylib loader at lua/fff/rust/init.lua hard-codes a relative
-- search off `dir`, so `dir` must be that store path verbatim.

local fff_dir = vim.env.FFF_NVIM_DIR
local nix_managed = fff_dir ~= nil and fff_dir ~= ""

return {
	{
		"dmtrKovalenko/fff.nvim",
		dir = nix_managed and fff_dir or nil,
		build = nix_managed and false or function()
			require("fff.download").download_or_build_binary()
		end,
		lazy = false,
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
