package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Pin is a pinned GitHub Actions workflow run.
type Pin struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	RunID    string `json:"run_id"`
	Hostname string `json:"hostname,omitempty"` // empty = gh default (github.com)
}

func (p Pin) Key() string {
	if p.Hostname != "" {
		return p.Hostname + "/" + p.Owner + "/" + p.Repo + "/" + p.RunID
	}
	return p.Owner + "/" + p.Repo + "/" + p.RunID
}

func (p Pin) RepoSlug() string { return p.Owner + "/" + p.Repo }

// RunStatus is the live state of a pinned run.
type RunStatus struct {
	Pin
	DisplayTitle string
	WorkflowName string
	Status       string // queued | in_progress | completed
	Conclusion   string // success | failure | cancelled | timed_out | ...
	HeadBranch   string
	Number       int
	UpdatedAt    time.Time
	URL          string
	FetchError   string
}

// runURLRE matches:
//
//	https://host/owner/repo/actions/runs/ID
//	owner/repo/actions/runs/ID
var runURLRE = regexp.MustCompile(
	`^(?:https?://([^/]+)/)?([^/]+)/([^/]+)/actions/runs/(\d+)`,
)

// gheHostname is the first non-github.com host found in ~/.config/gh/hosts.yml,
// used as the default when pasting short-form URLs without a hostname.
var gheHostname = sync.OnceValue(func() string {
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".config", "gh", "hosts.yml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimRight(line, " \r")
		// Top-level YAML keys are not indented and end with ':'
		if len(trimmed) > 0 && trimmed[0] != ' ' && trimmed[0] != '\t' && strings.HasSuffix(trimmed, ":") {
			host := strings.TrimSuffix(trimmed, ":")
			if host != "github.com" {
				return host
			}
		}
	}
	return ""
})

func parseRunURL(input string) (Pin, error) {
	input = strings.TrimSpace(input)
	m := runURLRE.FindStringSubmatch(input)
	if m == nil {
		return Pin{}, fmt.Errorf("expected owner/repo/actions/runs/ID or full URL")
	}
	hostname := m[1]
	if hostname == "github.com" {
		hostname = ""
	}
	// Apply detected GHE host when URL has no explicit hostname
	if hostname == "" {
		hostname = gheHostname()
	}
	return Pin{Hostname: hostname, Owner: m[2], Repo: m[3], RunID: m[4]}, nil
}

type ghRunJSON struct {
	DisplayTitle string    `json:"displayTitle"`
	WorkflowName string    `json:"workflowName"`
	Status       string    `json:"status"`
	Conclusion   string    `json:"conclusion"`
	HeadBranch   string    `json:"headBranch"`
	Number       int       `json:"number"`
	UpdatedAt    time.Time `json:"updatedAt"`
	URL          string    `json:"url"`
}

const ghFields = "status,conclusion,displayTitle,workflowName,headBranch,number,updatedAt,url"

func ghCmd(pin Pin, args ...string) *exec.Cmd {
	cmd := exec.Command("gh", args...)
	if pin.Hostname != "" {
		cmd.Env = append(os.Environ(), "GH_HOST="+pin.Hostname)
	}
	return cmd
}

func fetchRunStatus(pin Pin) RunStatus {
	args := []string{"run", "view", pin.RunID, "--repo", pin.RepoSlug(), "--json", ghFields}
	out, err := ghCmd(pin, args...).Output()
	if err != nil {
		msg := err.Error()
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			msg = strings.TrimSpace(string(ee.Stderr))
		}
		return RunStatus{Pin: pin, FetchError: msg}
	}

	var r ghRunJSON
	if err := json.Unmarshal(out, &r); err != nil {
		return RunStatus{Pin: pin, FetchError: "parse error: " + err.Error()}
	}

	return RunStatus{
		Pin:          pin,
		DisplayTitle: r.DisplayTitle,
		WorkflowName: r.WorkflowName,
		Status:       r.Status,
		Conclusion:   r.Conclusion,
		HeadBranch:   r.HeadBranch,
		Number:       r.Number,
		UpdatedAt:    r.UpdatedAt,
		URL:          r.URL,
	}
}

func openRunInBrowser(pin Pin) {
	_ = ghCmd(pin, "run", "view", pin.RunID, "--repo", pin.RepoSlug(), "--web").Start()
}
