package utils

func MinUint64(i, j uint64) uint64 {
	if i < j {
		return i
	}

	return j
}
