// Package version exposes the application version embedded in source installs and builds.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var source string

// String returns the version prepared for the next Git tag, without a v prefix.
func String() string { return strings.TrimSpace(source) }
