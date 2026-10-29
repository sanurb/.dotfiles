# cfpaste

Pi extension for creating Markdown pastes on Cloudflare Paste.

## Configuration

Set `PI_CFPASTE_ORIGIN` to the HTTPS origin of a Paste service you are authorized
to use, then reload Pi. Without it, this extension registers no commands. No
upstream corporate endpoint is selected automatically. `cloudflared` must be
installed and authorized for that origin. Invoking a paste command uploads the
selected Markdown to that service; review its content before sharing.

## Usage

```text
/cf-paste path/to/file.md
/cf-paste-last
```

`/cf-paste <path>` reads a `.md` or `.markdown` file relative to Pi's working directory. `/cf-paste-last` pastes the latest agent message on the active session branch. Both commands preserve the Markdown source and return a rendered URL ending in `/markdown`.

Authentication uses `cloudflared access token`. If Paste rejects the cached token, the extension runs `cloudflared access login`, waits for browser approval, and retries once.
