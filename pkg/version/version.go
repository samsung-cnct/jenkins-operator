/*
Copyright 2018 Samsung SDS Cloud Native Computing Team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package version

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var (
	version   string = ""
	gitCommit string = ""
	buildDate string = ""
)

type Info struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
	Compiler  string `json:"compiler"`
	Platform  string `json:"platform"`
}

// String returns info as a human-friendly version string.
func (info Info) String() string {
	return info.Version
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
	if version == "" {
		cmd = exec.Command("bash", "-c", "cat .versionfile")
		versionTemp, _ := cmd.CombinedOutput()
		version = strings.TrimSpace(string(versionTemp))
	}

	// These variables typically come from -ldflags settings and in
	// their absence fallback to the settings in pkg/version/base.go
	return Info{
		Version:   version,
		GitCommit: gitCommit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		Compiler:  runtime.Compiler,
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
