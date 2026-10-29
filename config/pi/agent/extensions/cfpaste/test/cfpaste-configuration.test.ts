import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { discoverAndLoadExtensions } from "@earendil-works/pi-coding-agent";

const extensionPath = fileURLToPath(new URL("../index.ts", import.meta.url));

test("paste commands require an explicit service origin and reject unsafe configuration", async () => {
	const cwd = await mkdtemp(join(tmpdir(), "pi-cfpaste-configuration-"));
	const previousOrigin = process.env.PI_CFPASTE_ORIGIN;
	try {
		delete process.env.PI_CFPASTE_ORIGIN;
		const disabled = await discoverAndLoadExtensions([extensionPath], cwd, join(cwd, ".agent"));
		assert.deepEqual(disabled.errors, []);
		assert.equal(disabled.extensions[0]?.commands.size, 0);

		for (const origin of ["not-a-url", "http://paste.example.test", "https://user:secret@paste.example.test", "https://paste.example.test/path", "https://paste.example.test/?token=secret"]) {
			process.env.PI_CFPASTE_ORIGIN = origin;
			const invalid = await discoverAndLoadExtensions([extensionPath], cwd, join(cwd, ".agent"));
			assert.equal(invalid.errors.length, 1);
			assert.match(invalid.errors[0]?.error ?? "", /Cloudflare Paste origin configuration/);
			assert.doesNotMatch(invalid.errors[0]?.error ?? "", /secret/);
			assert.equal(invalid.extensions.length, 0);
		}

		process.env.PI_CFPASTE_ORIGIN = "https://paste.example.test";
		const enabled = await discoverAndLoadExtensions([extensionPath], cwd, join(cwd, ".agent"));
		assert.deepEqual(enabled.errors, []);
		const extension = enabled.extensions[0];
		assert.ok(extension);
		assert.deepEqual([...extension.commands.keys()].sort(), ["cf-paste", "cf-paste-last"]);
	} finally {
		if (previousOrigin === undefined) delete process.env.PI_CFPASTE_ORIGIN;
		else process.env.PI_CFPASTE_ORIGIN = previousOrigin;
		await rm(cwd, { recursive: true, force: true });
	}
});
