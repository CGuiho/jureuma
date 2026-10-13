//go:build !linux

package runner

import (
	"errors"
	"os"
)

// memfdDocument is unavailable outside Linux; Run falls back to a 0600 file
// under <home>/run/<id>/.
func memfdDocument(data []byte) (*os.File, error) { return nil, errors.ErrUnsupported }
