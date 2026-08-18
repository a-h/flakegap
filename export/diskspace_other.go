//go:build !darwin && !linux

package export

import "errors"

func availableBytes(_ string) (uint64, error) {
	return 0, errors.ErrUnsupported
}
