package ngstate

import "C"
import (
	"fmt"

	"github.com/c0mm4nd/wasman"
)

func cp(ins *wasman.Instance, ptr uint32, data []byte) (uint32, error) {
	if len(ins.Memory.Value[ptr:]) < len(data) {
		return 0, fmt.Errorf("memory is not enough for data: %s", data)
	}

	l := copy(ins.Memory.Value[ptr:], data)
	return uint32(l), nil
}
