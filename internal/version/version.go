// Package version holds build-time version metadata injected via -ldflags.
package version

var (
	// Version is set via -ldflags "-X ...Version=vX.Y.Z" at release build time.
	Version = "dev"
	// Commit is set via -ldflags at release build time.
	Commit = "unknown"
)

// String returns a human-readable version string, e.g. "v0.1.0 (abcdef1)".
func String() string {
	return Version + " (" + Commit + ")"
}
