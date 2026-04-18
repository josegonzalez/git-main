package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "--initial-branch=main", dir},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func setupTestRepoWithRemote(t *testing.T) string {
	t.Helper()
	dir := setupTestRepo(t)
	remoteDir := t.TempDir()

	cmd := exec.Command("git", "clone", "--bare", dir, remoteDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bare clone: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("add remote: %v\n%s", err, out)
	}

	cmd = exec.Command("git", "fetch", "origin")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fetch: %v\n%s", err, out)
	}

	return dir
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(origDir) })
}

func TestGetDefaultBranchFromSymref(t *testing.T) {
	dir := setupTestRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "clone", "--bare", dir, remoteDir).Run()

	cmd := exec.Command("git", "remote", "add", "origin", remoteDir)
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "fetch", "origin")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	cmd.Dir = dir
	cmd.Run()

	chdir(t, dir)

	got := getDefaultBranch("origin")
	if got != "main" {
		t.Errorf("getDefaultBranch() = %q, want %q", got, "main")
	}
}

func TestGetDefaultBranchFromConfig(t *testing.T) {
	dir := setupTestRepo(t)

	cmd := exec.Command("git", "config", "init.defaultBranch", "develop")
	cmd.Dir = dir
	cmd.Run()

	chdir(t, dir)

	got := getDefaultBranch("origin")
	if got != "develop" {
		t.Errorf("getDefaultBranch() = %q, want %q", got, "develop")
	}
}

func TestGetDefaultBranchFallbackMain(t *testing.T) {
	dir := setupTestRepo(t)

	cmd := exec.Command("git", "config", "--unset", "init.defaultBranch")
	cmd.Dir = dir
	cmd.Run()

	chdir(t, dir)

	got := getDefaultBranch("origin")
	if got != "main" {
		t.Errorf("getDefaultBranch() = %q, want %q", got, "main")
	}
}

func TestGetDefaultBranchFallbackMaster(t *testing.T) {
	// Isolate from user's global and system git config
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", "--initial-branch=master", dir},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %v\n%s", args, err, out)
		}
	}

	chdir(t, dir)

	got := getDefaultBranch("origin")
	if got != "master" {
		t.Errorf("getDefaultBranch() = %q, want %q", got, "master")
	}
}

func TestIsInsideGitRepo(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)

	if !isInsideGitRepo() {
		t.Error("isInsideGitRepo() = false, want true")
	}
}

func TestIsInsideGitRepoFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if isInsideGitRepo() {
		t.Error("isInsideGitRepo() = true, want false")
	}
}

func TestGetCurrentBranch(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)

	got, err := getCurrentBranch()
	if err != nil {
		t.Fatalf("getCurrentBranch() error = %v", err)
	}
	if got != "main" {
		t.Errorf("getCurrentBranch() = %q, want %q", got, "main")
	}
}

func TestGetCurrentBranchFeature(t *testing.T) {
	dir := setupTestRepo(t)

	cmd := exec.Command("git", "checkout", "-b", "feature")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout -b feature: %v\n%s", err, out)
	}

	chdir(t, dir)

	got, err := getCurrentBranch()
	if err != nil {
		t.Fatalf("getCurrentBranch() error = %v", err)
	}
	if got != "feature" {
		t.Errorf("getCurrentBranch() = %q, want %q", got, "feature")
	}
}

func TestHasChangesToStashClean(t *testing.T) {
	dir := setupTestRepo(t)
	chdir(t, dir)

	if hasChangesToStash() {
		t.Error("hasChangesToStash() = true, want false for clean repo")
	}
}

func TestHasChangesToStashUnstaged(t *testing.T) {
	dir := setupTestRepo(t)

	// Create a tracked file first
	filePath := filepath.Join(dir, "file.txt")
	os.WriteFile(filePath, []byte("hello"), 0644)
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "add file")
	cmd.Dir = dir
	cmd.Run()

	// Modify it (unstaged change)
	os.WriteFile(filePath, []byte("world"), 0644)

	chdir(t, dir)

	if !hasChangesToStash() {
		t.Error("hasChangesToStash() = false, want true for unstaged changes")
	}
}

func TestHasChangesToStashStaged(t *testing.T) {
	dir := setupTestRepo(t)

	filePath := filepath.Join(dir, "file.txt")
	os.WriteFile(filePath, []byte("hello"), 0644)
	cmd := exec.Command("git", "add", "file.txt")
	cmd.Dir = dir
	cmd.Run()

	chdir(t, dir)

	if !hasChangesToStash() {
		t.Error("hasChangesToStash() = false, want true for staged changes")
	}
}

func TestPullRebaseWhenAlreadyOnTrunk(t *testing.T) {
	dir := setupTestRepoWithRemote(t)

	// Find the bare remote path from git config
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("get remote url: %v", err)
	}
	bareDir := strings.TrimSpace(string(out))

	// Clone the bare repo into a second working copy and push a new commit
	secondClone := t.TempDir()
	cmd = exec.Command("git", "clone", bareDir, secondClone)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("second clone: %v\n%s", err, out)
	}

	newFile := filepath.Join(secondClone, "new.txt")
	os.WriteFile(newFile, []byte("from second clone"), 0644)
	for _, args := range [][]string{
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "add", "new.txt"},
		{"git", "commit", "-m", "remote commit"},
		{"git", "push", "origin", "main"},
	} {
		cmd = exec.Command(args[0], args[1:]...)
		cmd.Dir = secondClone
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}

	// Now in the original repo, we're on main (the trunk branch).
	// Verify the new file doesn't exist yet.
	chdir(t, dir)
	localFile := filepath.Join(dir, "new.txt")
	if _, err := os.Stat(localFile); err == nil {
		t.Fatal("new.txt should not exist before pull")
	}

	// Run pull --rebase (simulating what main() does when already on trunk)
	if err := runGit("pull", "--rebase", "origin", "main"); err != nil {
		t.Fatalf("pull --rebase failed: %v", err)
	}

	// The new file should now exist
	if _, err := os.Stat(localFile); err != nil {
		t.Error("new.txt should exist after pull --rebase on trunk branch")
	}
}
