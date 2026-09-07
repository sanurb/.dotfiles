-- Headless integration test: actual visual selection and file I/O, stubbed
-- Herdr process so validation cannot send text to a running session.
local runtime = vim.fn.tempname()
vim.fn.mkdir(runtime, "p")
vim.env.XDG_RUNTIME_DIR = runtime
vim.env.HERDR_ENV = nil
vim.env.HERDR_BIN_PATH = nil
local notifications = {}
vim.notify = function(message)
	table.insert(notifications, message)
end
local invocation
vim.fn.executable = function(command)
	return command == "herdr" and 1 or 0
end
vim.fn.jobstart = function(args, options)
	invocation = { args = args, options = options }
	return 1
end
dofile(assert(arg[1], "expected annotation mapping path"))
local mapping
for _, map in ipairs(vim.api.nvim_get_keymap("x")) do
	if map.desc == "Annotate selection in Herdr" then
		mapping = map.callback
	end
end
assert(mapping, "mapping not registered")
mapping()
assert(not invocation and #notifications == 1, "outside-Herdr guard failed")

vim.env.HERDR_ENV = "1"
vim.api.nvim_buf_set_lines(0, 0, -1, false, { "hello world" })
vim.fn.setreg("z", "saved register")
local unnamed = vim.fn.getreg('"')
vim.cmd("normal! gg0v4l")
mapping()
local path = runtime .. "/herdr-annotate-" .. vim.uv.getuid() .. "/selection"
assert(vim.fn.readfile(path)[1] == "hello", "visual selection was not handed off")
assert(vim.fn.getreg("z") == "saved register", "named register changed")
assert(vim.fn.getreg('"') == unnamed, "unnamed register changed")
assert(vim.uv.fs_stat(path).mode % 512 == 384, "selection is not private")
assert(vim.deep_equal(invocation.args, { "herdr", "plugin", "action", "invoke", "annotate.capture" }))

local first_invocation = invocation
vim.cmd("normal! gg0v1l")
mapping()
assert(invocation == first_invocation, "pending selection was overwritten")
assert(vim.fn.readfile(path)[1] == "hello")
assert(#notifications == 2, "pending selection error was not reported")

assert(vim.uv.fs_utime(path, os.time() - 30, os.time() - 30))
vim.cmd("normal! gg0v1l")
mapping()
assert(invocation ~= first_invocation, "expired selection blocked the next invocation")
assert(vim.fn.readfile(path)[1] == "he")

invocation.options.on_exit(1, 1)
vim.wait(100, function()
	return #notifications == 3
end)
assert(not vim.uv.fs_lstat(path), "failed invocation left a stale selection")
vim.fn.delete(runtime, "rf")
print("Herdr annotation: selection, registers, permissions, and failure handling pass")
