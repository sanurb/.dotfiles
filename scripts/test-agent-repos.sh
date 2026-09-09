#!/usr/bin/env bash
# Run with: scripts/run-in-dev-shell.sh bash scripts/test-agent-repos.sh
set -euo pipefail

agent_repos_script=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/config/bin/agent-repos
fixture_root=$(mktemp -d)
trap 'rm -rf "$fixture_root"' EXIT

export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME="Fixture Author"
export GIT_AUTHOR_EMAIL="fixture@example.invalid"
export GIT_COMMITTER_NAME="$GIT_AUTHOR_NAME"
export GIT_COMMITTER_EMAIL="$GIT_AUTHOR_EMAIL"

git init -q --initial-branch=main "$fixture_root/upstream"
printf 'reference source\n' >"$fixture_root/upstream/reference.txt"
git -C "$fixture_root/upstream" add reference.txt
git -C "$fixture_root/upstream" commit -qm "Add reference fixture"

add_reference() {
    bash "$agent_repos_script" add "$fixture_root/upstream" --name reference --branch main
}

assert_reference() {
    [[ "$(cat repos/reference/reference.txt)" == "reference source" ]]
    [[ "$(git ls-tree -r --name-only HEAD repos/reference)" == "repos/reference/reference.txt" ]]
    [[ "$(git stash list)" == "" ]]
}

test_empty_repository() (
    git init -q --initial-branch=main "$fixture_root/empty"
    cd "$fixture_root/empty"
    add_reference
    assert_reference
    [[ "$(git ls-tree -r --name-only HEAD^1)" == "" ]]
    [[ "$(git rev-list --first-parent --count HEAD)" == "2" ]]
)

test_staged_changes() (
    git init -q --initial-branch=main "$fixture_root/staged"
    cd "$fixture_root/staged"
    printf 'staged candidate content\n' >candidate.txt
    git add candidate.txt
    printf 'unstaged candidate content\n' >>candidate.txt
    printf 'untracked content\n' >untracked.txt
    add_reference
    assert_reference
    [[ "$(git ls-tree -r --name-only HEAD^1)" == "" ]]
    [[ "$(git ls-tree -r --name-only HEAD candidate.txt untracked.txt)" == "" ]]
    [[ "$(git show :candidate.txt)" == "staged candidate content" ]]
    [[ "$(cat candidate.txt)" == $'staged candidate content\nunstaged candidate content' ]]
    [[ "$(cat untracked.txt)" == "untracked content" ]]
    [[ "$(git status --porcelain -- candidate.txt)" == "AM candidate.txt" ]]
)

test_existing_repository() (
    git init -q --initial-branch=main "$fixture_root/existing"
    cd "$fixture_root/existing"
    printf 'existing content\n' >existing.txt
    git add existing.txt
    git commit -qm "Initialize existing fixture"
    initial_head=$(git rev-parse HEAD)
    add_reference
    assert_reference
    [[ "$(git rev-parse HEAD^1)" == "$initial_head" ]]
    [[ "$(cat existing.txt)" == "existing content" ]]
)

bash -n "$agent_repos_script"
failures=0
for test_case in test_empty_repository test_staged_changes test_existing_repository; do
    # A fresh Bash process keeps errexit active inside each test body.
    export agent_repos_script fixture_root
    export -f add_reference assert_reference "$test_case"
    if bash -e -u -o pipefail -c "$test_case" >"$fixture_root/$test_case.log" 2>&1; then
        printf 'PASS %s\n' "$test_case"
    else
        printf 'FAIL %s\n' "$test_case" >&2
        cat "$fixture_root/$test_case.log" >&2
        failures=$((failures + 1))
    fi
done
[[ "$failures" == "0" ]]
