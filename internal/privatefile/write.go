// Package privatefile writes local user data with owner-only permissions where
// the operating system supports Unix file modes.
package privatefile

import "os"

// Write saves data with mode 0600. On Unix-like systems this limits access to
// the current user, including when the file already exists. Windows does not
// map Unix permission bits to a restrictive ACL, so the parent directory's
// Windows permissions still determine access there.
func Write(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
