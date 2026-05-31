package utils

import (
	"unsafe"
)

// String safely converts a byte slice to a string with zero allocations.
// WARNING: Do not use this for strings that will be permanently cached
// (e.g. GlobalSymbol.Name) as it keeps the underlying byte slice in memory.
func String(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	return unsafe.String(unsafe.SliceData(b), len(b))
}
