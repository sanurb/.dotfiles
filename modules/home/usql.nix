{ pkgs, ... }:
{
  # usql — universal SQL client. One static Go binary that speaks ~20
  # engines behind a URL DSN (`sqlite3:app.db`, `pg://host/db`,
  # `mysql://…`), so the invocation shape is identical no matter which
  # database is on the other end. nixpkgs ships the full-driver build:
  # postgres, mysql, sqlite3, sqlserver, oracle, clickhouse, duckdb,
  # snowflake and bigquery are all present (`usql -c '\drivers'`).
  #
  # Chosen over the dbcli family (pgcli/mycli/litecli) because those are
  # REPLs built for humans — readline, autocomplete, a pager — and their
  # value evaporates under a script. usql is the opposite trade:
  #
  #   usql -J -c "SELECT …" pg://host/db   # JSON on stdout
  #   usql -C -c "SELECT …" sqlite3:app.db # CSV on stdout
  #
  # It honours the contract non-interactive callers actually need — a
  # failed query exits 1, stdout stays empty, and the diagnostic goes to
  # stderr — so `set -e` and an agent's exit-code check both behave.
  # Its JSON is also correctly typed (numbers stay numbers), which
  # matters when the next stage is `| jaq`; duckdb's `-json` stringifies
  # HUGEINT, so `sum(x)` there needs an explicit `::BIGINT` cast.
  #
  # Complements rather than replaces duckdb, should that land later:
  # duckdb is the engine for querying files with no server, usql is the
  # client for databases that already exist.
  #
  # No `programs.usql` HM module exists; direct home.packages install
  # follows the jaq/gron pattern. Nothing user-global to manage —
  # connection strings are per-invocation, and credentials belong in the
  # environment, not in a dotfile.
  home.packages = [ pkgs.usql ];
}
