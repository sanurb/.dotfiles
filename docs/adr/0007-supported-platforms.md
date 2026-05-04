# Supported platforms: macOS and Linux only

`dots` targets `darwin/{amd64,arm64}` and `linux/{amd64,arm64}`. No
Windows artifact ships and the realization layer does not declare
Windows-compatible derivations. `nh`, moon, and Home Manager have no
Windows ports; supporting Windows would mean vendoring or forking
three upstreams. WSL2 is the supported Windows path — both the CLI
and `dots apply` work inside it.
