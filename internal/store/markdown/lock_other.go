//go:build !unix && !windows

package markdown

import (
	"fmt"
	"os"
)

func tryFileLock(file *os.File) (bool, error) {
	return false, fmt.Errorf("waiting store requires OS file locking on this platform")
}
