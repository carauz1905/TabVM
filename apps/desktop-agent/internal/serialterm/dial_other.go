//go:build !windows

package serialterm

import (
	"errors"
	"io"
)

// Dial is unsupported off Windows: the serial bridge relies on Windows named
// pipes. This stub keeps the package building on other platforms (e.g. CI).
func Dial(pipe string) (io.ReadWriteCloser, error) {
	return nil, errors.New("serial console pipe is only supported on Windows")
}
