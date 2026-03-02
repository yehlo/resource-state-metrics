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

// Package metricutil provides shared metric utilities for the resource-state-metrics project.
package metricutil

import "strings"

// MetricKind represents the type of metric (gauge or counter).
type MetricKind string

const (
	// MetricKindGauge represents a gauge metric type.
	MetricKindGauge MetricKind = "gauge"
	// MetricKindCounter represents a counter metric type.
	MetricKindCounter MetricKind = "counter"
)

// SupportedMetricKinds contains all valid metric kinds.
var SupportedMetricKinds = []MetricKind{
	MetricKindGauge,
	MetricKindCounter,
}

// IsValidMetricKind checks if the given kind is a supported metric kind.
func IsValidMetricKind(kind string) bool {
	for _, supported := range SupportedMetricKinds {
		if string(supported) == kind {
			return true
		}
	}

	return false
}

// SupportedMetricKindsString returns a comma-separated string of supported metric kinds.
func SupportedMetricKindsString() string {
	kinds := make([]string, len(SupportedMetricKinds))
	for i, kind := range SupportedMetricKinds {
		kinds[i] = "'" + string(kind) + "'"
	}

	return strings.Join(kinds, ", ")
}

// SanitizeLabelKey converts a string to a valid Prometheus label key.
// It replaces non-alphanumeric characters (except underscore) with underscores.
// If the first character is a digit, it's replaced with an underscore.
func SanitizeLabelKey(key string) string {
	var result strings.Builder
	for i, r := range key {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (r >= '0' && r <= '9' && i > 0) {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}

	return result.String()
}
