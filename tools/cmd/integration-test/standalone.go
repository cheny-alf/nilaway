//  Copyright (c) 2023 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// StandaloneDriver implements Driver for running NilAway as a standalone binary.
type StandaloneDriver struct{}

// Run runs NilAway as a standalone binary on the test project and returns the diagnostics.
func (d *StandaloneDriver) Run(dir string) (map[Position]string, error) {
	// Build NilAway first.
	if out, err := exec.Command("make", "build").CombinedOutput(); err != nil {
		return nil, fmt.Errorf("build NilAway: %w: %q", err, string(out))
	}

	// Run the NilAway binary on the integration test project, with redirects to an internal buffer.
	cmd := exec.Command(filepath.Join("..", "..", "bin", "nilaway"),
		"-json", "-pretty-print=false",
		// Disable group error messages to make the output accurate for comparisons.
		"-group-error-messages=false",
		"./...",
	)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run nilaway: %w\n%s", err, string(out))
	}

	// Parse the diagnostics.
	type diagnostic struct {
		Posn    string `json:"posn"`
		Message string `json:"message"`
	}
	// pkg name -> "nilaway" -> list of diagnostics.
	var result map[string]map[string][]diagnostic
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("decode nilaway output: %w", err)
	}

	collected := make(map[Position]string)
	for _, m := range result {
		diagnostics, ok := m["nilaway"]
		if !ok {
			return nil, fmt.Errorf("expect \"nilaway\" key in result, got %+v", m)
		}
		for _, d := range diagnostics {
			parts := strings.Split(d.Posn, ":")
			if len(parts) != 3 {
				return nil, fmt.Errorf("expect 3 parts in position string, got %+v", d)
			}
			// Convert diagnostic output from NilAway to canonical form.
			line, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("convert line number: %w", err)
			}
			pos := Position{Filename: parts[0], Line: line}
			if current, ok := collected[pos]; ok {
				return nil, fmt.Errorf("multiple diagnostics on the same line not supported, current: %q, got: %q", current, d.Message)
			}
			collected[pos] = d.Message
		}
	}

	return collected, nil
}
