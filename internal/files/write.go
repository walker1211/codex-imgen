package files

import "os"

func EnsureJobDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
