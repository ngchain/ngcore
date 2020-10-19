package ngstate

import "C"
import (
	"fmt"
	"github.com/c0mm4nd/wasman"
	"unsafe"
)

func alloc(ins *wasman.Instance, length int) (uint32, error) {
	ret, _, err := ins.CallExportedFunc("allocate", uint64(length))

	if err != nil {
		return 0, err
	}

	//avoid panic
	if len(ret) != 1 {
		return 0, wasman.ErrInvalidSign // the func's signature is incorrect
	}

	return uint32(ret[0]), nil
}

func cp(ins *wasman.Instance, ptr uint32, data []byte) error {
	if len(ins.Memory.Value[ptr:]) < len(data) {
		return fmt.Errorf("memory is not enough for data: %s", data)
	}

	copy(ins.Memory.Value[ptr:], data)
	return nil
}

func writeToMem(ins *wasman.Instance, data []byte) (uint32, error) {
	ptr, err := alloc(ins, len(data)+1)
	if err != nil {
		return 0, err
	}

	err = cp(ins, ptr, data)
	if err != nil {
		return 0, err
	}

	return ptr, nil
}

func strFromPtr(ins *wasman.Instance, ptr uint32) string {
	return C.GoString((*C.char)(unsafe.Pointer(&ins.Memory.Value[ptr])))
}
