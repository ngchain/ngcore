package utils

import "bytes"

func MinUint64(i, j uint64) uint64 {
	if i < j {
		return i
	}

	return j
}

func MaxUint64(i, j uint64) uint64 {
	if i > j {
		return i
	}

	return j
}

func BytesListEquals(p1, p2 [][]byte) bool {
	if len(p1) != len(p2) {
		return false
	}

	for i := range p1 {
		if !bytes.Equal(p1[i], p2[i]) {
			return false
		}
	}

	return true
}
