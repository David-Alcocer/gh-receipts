package cli

import "runtime/debug"

// fallbackVersion is reported when the binary has no version stamped by the Go
// toolchain.
const fallbackVersion = "dev"

// versionFromBuildInfo returns the module version recorded at build time. The
// Go toolchain derives it from the Git tag on the commit being built, so
// release binaries need no -ldflags.
func versionFromBuildInfo(info *debug.BuildInfo, ok bool) string {
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return fallbackVersion
	}
	return info.Main.Version
}
