package version

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var (
	gitVersion   string = ""
	gitCommit    string = ""
	gitTreeState string = "dirty"
	buildDate    string = ""
)

type Info struct {
	GitVersion   string `json:"gitVersion"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Compiler     string `json:"compiler"`
	Platform     string `json:"platform"`
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	return info.GitVersion
}

// Get returns the overall codebase version. It's for detecting
// what code a binary was built from.
func Get() Info {
	var cmd *exec.Cmd

	// For Local development
	if gitCommit == "" {
		cmd = exec.Command("bash", "-c", "git rev-parse HEAD")
		gitCommitTemp, _ := cmd.CombinedOutput()
		gitCommit = strings.TrimSpace(string(gitCommitTemp))
	}
	if gitVersion == "" {
		cmd = exec.Command("bash", "-c", "git describe --tags --abbrev=0 --exact-match 2>/dev/null")
		gitVersionTemp, _ := cmd.CombinedOutput()
		gitVersion = strings.TrimSpace(string(gitVersionTemp))
	}

	// These variables typically come from -ldflags settings and in
	// their absence fallback to the settings in pkg/version/base.go
	return Info{
		GitVersion:   gitVersion,
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		BuildDate:    buildDate,
		GoVersion:    runtime.Version(),
		Compiler:     runtime.Compiler,
		Platform:     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
