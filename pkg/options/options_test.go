/*
Copyright 2026 The Kubernetes resource-state-metrics Authors.

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

package options

import (
	"os"
	"strconv"
	"testing"

	"k8s.io/klog/v2"
)

// Tests utilizing t.Setenv cannot be run in t.Parallel().
func TestOptions_Read(t *testing.T) {
	// Define the command-line arguments.
	originalMainPortNumber := 4242
	os.Args = []string{
		"cmd",
		"--main-port", strconv.Itoa(originalMainPortNumber), // This will *not* be overridden as it was explicitly set.
	}

	// Override the --self-port flag with the RSM_SELF_PORT environment variable.
	overriddenSelfPortNumber := 5678
	t.Setenv("RSM_SELF_PORT", strconv.Itoa(overriddenSelfPortNumber))

	// Override the --main-port flag with the RSM_MAIN_PORT environment variable.
	overriddenMainPortNumber := 1234
	t.Setenv("RSM_MAIN_PORT", strconv.Itoa(overriddenMainPortNumber))

	// Check if the flags were overridden by their corresponding environment variables.
	o := NewOptions(klog.NewKlogr())
	o.Read()

	if *o.SelfPort != overriddenSelfPortNumber {
		t.Fatalf("expected %d, got %d", overriddenSelfPortNumber, *o.SelfPort)
	}

	if *o.MainPort != originalMainPortNumber {
		t.Fatalf("expected %d, got %d", originalMainPortNumber, *o.MainPort)
	}
}
