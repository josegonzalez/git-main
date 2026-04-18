package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	flag "github.com/spf13/pflag"
)

// Version is set via ldflags at build time.
var Version = "dev"

func getDefaultBranch(remoteName string) string {
	// 1. Query the remote directly for its HEAD (avoids stale local symrefs)
	out, err := exec.Command("git", "ls-remote", "--symref", remoteName, "HEAD").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "ref: refs/heads/") {
				ref := strings.TrimPrefix(line, "ref: refs/heads/")
				if i := strings.Index(ref, "\t"); i >= 0 {
					return ref[:i]
				}
			}
		}
	}

	// 2. Fall back to local symref (works offline)
	out, err = exec.Command("git", "symbolic-ref", fmt.Sprintf("refs/remotes/%s/HEAD", remoteName)).Output()
	if err == nil {
		ref := strings.TrimSpace(string(out))
		if i := strings.LastIndex(ref, "/"); i >= 0 {
			return ref[i+1:]
		}
	}

	// 3. Git config init.defaultBranch
	out, err = exec.Command("git", "config", "init.defaultBranch").Output()
	if err == nil {
		if branch := strings.TrimSpace(string(out)); branch != "" {
			return branch
		}
	}

	// 4. Check if main or master exist locally
	for _, branch := range []string{"main", "master"} {
		if exec.Command("git", "rev-parse", "--verify", branch).Run() == nil {
			return branch
		}
	}

	return "main"
}

func getCurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("failed to determine current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func isInsideGitRepo() bool {
	return exec.Command("git", "rev-parse", "--git-dir").Run() == nil
}

func hasChangesToStash() bool {
	// Staged changes?
	if exec.Command("git", "diff", "--cached", "--quiet").Run() != nil {
		return true
	}
	// Unstaged changes to tracked files?
	if exec.Command("git", "diff", "--quiet").Run() != nil {
		return true
	}
	return false
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: git main\n\n")
		fmt.Fprintf(os.Stderr, "Switch to the default branch, rebasing on the remote.\n\n")
		flag.PrintDefaults()
	}
	remote := flag.StringP("remote", "r", "origin", "git remote to use")
	version := flag.BoolP("version", "v", false, "print version")
	flag.Parse()

	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}

	if flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "unexpected arguments; usage: git main")
		os.Exit(1)
	}

	if !isInsideGitRepo() {
		fmt.Fprintln(os.Stderr, "fatal: not a git repository")
		os.Exit(1)
	}

	mainBranch := getDefaultBranch(*remote)

	currentBranch, err := getCurrentBranch()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	didStash := false
	if hasChangesToStash() {
		if err := runGit("stash"); err != nil {
			fmt.Fprintln(os.Stderr, "failed to stash changes")
			os.Exit(1)
		}
		didStash = true
	}

	if currentBranch != mainBranch {
		if err := runGit("checkout", mainBranch); err != nil {
			fmt.Fprintf(os.Stderr, "failed to checkout %s\n", mainBranch)
			if didStash {
				runGit("stash", "pop")
			}
			os.Exit(1)
		}
	}

	if err := runGit("pull", "--rebase", *remote, mainBranch); err != nil {
		fmt.Fprintf(os.Stderr, "failed to pull --rebase %s %s\n", *remote, mainBranch)
		if didStash {
			runGit("stash", "pop")
		}
		os.Exit(1)
	}

	if didStash {
		if err := runGit("stash", "pop"); err != nil {
			fmt.Fprintln(os.Stderr, "warning: failed to pop stash (your changes are still in git stash)")
			os.Exit(1)
		}
	}
}
