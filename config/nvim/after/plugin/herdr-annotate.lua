-- Adapted from dmmulroy/.dotfiles fd84f529. Keep the upstream handoff
-- protocol, but preserve registers and report missing prerequisites.
vim.keymap.set("x", "<leader>a", function()
	local herdr = vim.env.HERDR_BIN_PATH
	if not herdr or herdr == "" then
		herdr = "herdr"
	end
	if vim.env.HERDR_ENV ~= "1" or vim.fn.executable(herdr) ~= 1 then
		vim.notify("Annotation requires a Herdr pane with the Annotate plugin installed", vim.log.levels.WARN)
		return
	end

	local register = vim.fn.getreginfo("z")
	local unnamed = vim.fn.getreginfo('"')
	vim.cmd('normal! "zy')
	local selection = vim.fn.getreg("z")
	vim.fn.setreg("z", register)
	vim.fn.setreg('"', unnamed)

	local base = vim.env.XDG_RUNTIME_DIR
	if not base or base == "" then
		base = vim.uv.os_tmpdir()
	end
	local dir = base .. "/herdr-annotate-" .. vim.uv.getuid()
	local path = dir .. "/selection"
	local ok, err = pcall(function()
		vim.fn.mkdir(dir, "p", "0700")
		local info = vim.uv.fs_lstat(dir)
		assert(info and info.type == "directory" and info.uid == vim.uv.getuid(), "unsafe handoff directory")
		assert(vim.uv.fs_chmod(dir, 448)) -- 0700
		local pending = vim.uv.fs_lstat(path)
		-- Match the plugin's 15-second expiry so an interrupted invocation
		-- cannot permanently block subsequent selections.
		if pending and pending.type == "file" and pending.uid == vim.uv.getuid() and os.time() - pending.mtime.sec > 15 then
			assert(vim.uv.fs_unlink(path))
		end
		-- Exclusive creation avoids following a stale symlink or overwriting
		-- another pane's pending selection. The plugin consumes this file.
		local fd = assert(vim.uv.fs_open(path, "wx", 384)) -- 0600
		local written, write_err = vim.uv.fs_write(fd, selection, 0)
		vim.uv.fs_close(fd)
		if not written then
			vim.uv.fs_unlink(path)
			error(write_err)
		end
	end)
	if not ok then
		vim.notify("Could not hand selection to Herdr: " .. tostring(err), vim.log.levels.ERROR)
		return
	end
	local job = vim.fn.jobstart({ herdr, "plugin", "action", "invoke", "annotate.capture" }, {
		on_exit = function(_, code)
			if code ~= 0 then
				vim.uv.fs_unlink(path)
				vim.schedule(function()
					vim.notify("Herdr annotation failed; check that Annotate is installed", vim.log.levels.ERROR)
				end)
			end
		end,
	})
	if job <= 0 then
		vim.uv.fs_unlink(path)
		vim.notify("Could not start Herdr annotation", vim.log.levels.ERROR)
	end
end, { desc = "Annotate selection in Herdr" })
