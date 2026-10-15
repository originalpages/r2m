package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const nameRe = `[A-Za-z0-9_][A-Za-z0-9_.-]*`

var (
	shortRe = regexp.MustCompile(`^(` + nameRe + `)/(` + nameRe + `)$`)
	urlRe   = regexp.MustCompile(`^https://(?:www\.)?github\.com/(` + nameRe + `)/(` + nameRe + `?)(?:\.git)?/?$`)
)

// GitHubRepo is a parsed GitHub reference.
type GitHubRepo struct {
	Owner, Repo, URL string
}

// ParseGitHubSpec strictly parses `owner/repo` or an https GitHub URL.
func ParseGitHubSpec(input string) (GitHubRepo, bool) {
	m := shortRe.FindStringSubmatch(input)
	if m == nil {
		m = urlRe.FindStringSubmatch(input)
	}
	if m == nil {
		return GitHubRepo{}, false
	}
	owner, repo := m[1], strings.TrimSuffix(m[2], ".git")
	if repo == "" || repo == "." || repo == ".." || owner == "." || owner == ".." {
		return GitHubRepo{}, false
	}
	return GitHubRepo{owner, repo, fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)}, true
}

// Resolved is a local directory to analyse plus a cleanup hook.
type Resolved struct {
	Root string
	Name string // folder name or owner/repo
	// SourceURL is the GitHub URL when the input was a GitHub reference.
	SourceURL string
	// Arg is the input as typed by the user.
	Arg     string
	Cleanup func()
}

// ResolveInput resolves a local directory, or shallow-clones a GitHub repo into a temp dir.
func ResolveInput(input string) (*Resolved, error) {
	if st, err := os.Stat(input); err == nil {
		if !st.IsDir() {
			return nil, fmt.Errorf("not a directory: %s", input)
		}
		root, err := filepath.Abs(input)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(root)
		return &Resolved{Root: root, Name: name, Cleanup: func() {}}, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	gh, ok := ParseGitHubSpec(input)
	if !ok {
		return nil, fmt.Errorf("%q is not a folder here and not a GitHub project (use a folder path, owner/repo or https://github.com/owner/repo)", input)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return nil, fmt.Errorf("git is required to analyse GitHub repositories: install it from https://git-scm.com")
	}
	tmp, err := os.MkdirTemp("", "r2m-")
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	dest := filepath.Join(tmp, "repo")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--quiet", "--", gh.URL, dest)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		return nil, fmt.Errorf("could not download %s/%s: %s\n(Only public GitHub projects work. For a private one, clone it yourself and run r2m on the folder.)",
			gh.Owner, gh.Repo, strings.TrimSpace(string(out)))
	}
	return &Resolved{Root: dest, Name: gh.Owner + "/" + gh.Repo, SourceURL: "https://github.com/" + gh.Owner + "/" + gh.Repo, Cleanup: cleanup}, nil
}
