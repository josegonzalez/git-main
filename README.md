# git-main

Switch to the default branch, rebasing on the remote.

## Installation

```sh
go install github.com/josegonzalez/git-main@latest
```

Or build from source:

```sh
git clone https://github.com/josegonzalez/git-main.git
cd git-main
make install
```

## Usage

```sh
git main
```

Stashes uncommitted changes, checks out the default branch,
pulls with rebase from the remote, and pops the stash.

### Default branch detection

Detected using (in order):

1. `git ls-remote --symref <remote> HEAD`
2. `git symbolic-ref refs/remotes/<remote>/HEAD`
3. `git config init.defaultBranch`
4. Local existence of `main` or `master`
5. Fallback: `main`

### Flags

```text
-r, --remote   git remote to use (default "origin")
-v, --version  print version
-h, --help     print help
```

## License

MIT
