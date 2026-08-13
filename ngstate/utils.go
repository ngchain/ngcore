package ngstate

import (
	"github.com/c0mm4nd/wasman"
	"github.com/pkg/errors"
)

var ErrOutOfMem = errors.New("out of the allocated memory")

// readMem returns the [ptr, ptr+size) slice of the instance's linear
// memory, bounds-checked so a contract can never make the host read
// outside the sandbox
func readMem(ins *wasman.Instance, ptr, size uint32) ([]byte, error) {
	if ins.Memory == nil {
		return nil, errors.Wrap(ErrOutOfMem, "contract has no memory")
	}

	end := uint64(ptr) + uint64(size)
	if end > uint64(len(ins.Memory.Value)) {
		return nil, errors.Wrapf(ErrOutOfMem, "read [%d, %d) exceeds memory size %d", ptr, end, len(ins.Memory.Value))
	}

	return ins.Memory.Value[ptr:end], nil
}

// cp copies data into the instance's linear memory at ptr, bounds-checked
func cp(ins *wasman.Instance, ptr uint32, data []byte) (uint32, error) {
	if ins.Memory == nil {
		return 0, errors.Wrap(ErrOutOfMem, "contract has no memory")
	}

	end := uint64(ptr) + uint64(len(data))
	if end > uint64(len(ins.Memory.Value)) {
		return 0, errors.Wrapf(ErrOutOfMem, "write [%d, %d) exceeds memory size %d", ptr, end, len(ins.Memory.Value))
	}

	l := copy(ins.Memory.Value[ptr:], data)

	return uint32(l), nil
}
