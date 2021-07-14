package ngstate

import "C"
import (
	"github.com/c0mm4nd/wasman"
	"github.com/pkg/errors"
)

var ErrOutOfMem = errors.New("out of the allocated memory")

func cp(ins *wasman.Instance, ptr uint32, data []byte) (uint32, error) {
	if len(ins.Memory.Value[ptr:]) < len(data) {
		return 0, errors.Wrapf(ErrOutOfMem, "memory is not enough for data: %s", data)
	}

	l := copy(ins.Memory.Value[ptr:], data)
	return uint32(l), nil
}
