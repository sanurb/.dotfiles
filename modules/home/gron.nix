{ pkgs, ... }:
{
  # gron — flattens JSON into discrete, greppable assignments
  # (`json.users[0].email = "…";`) and reverses the transform with
  # `gron --ungron`. It does not query or transform like jaq; it makes
  # JSON *line-oriented*, which is the point. An agent (or a plain `rg`)
  # working over `curl … | gron | rg user.email` gets one line and the
  # exact path to it, instead of paying for a whole pretty-printed blob as
  # tokens — and a `gron a.json | diff - <(gron b.json)` shows precisely
  # which leaf changed, which a structural JSON diff buries.
  #
  # Rounds out the structured-data tier alongside jaq (JSON query/transform
  # via a filter language, modules/home/jaq.nix), yq (YAML/TOML/XML,
  # modules/home/yq.nix), and htmlq (HTML, modules/home/htmlq.nix): jaq is
  # the scalpel when you know the filter; gron is the grep when you don't.
  #
  # No `programs.gron` HM module exists; direct home.packages install
  # follows the jaq/ast-grep pattern. Nothing user-global to manage.
  home.packages = [ pkgs.gron ];
}
