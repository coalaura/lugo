package utils

const (
	FnvOffset64 = 14695981039346656037
	FnvPrime64  = 1099511628211
)

// HashBytes implements the FNV-1a 64-bit hash algorithm for zero-alloc map keys
func HashBytes(b []byte) uint64 {
	var hash uint64 = FnvOffset64

	if len(b) > 0 {
		_ = b[len(b)-1] // BCE

		for i := range b {
			hash ^= uint64(b[i])
			hash *= FnvPrime64
		}
	}

	return hash
}

// HashString implements the FNV-1a 64-bit hash algorithm for zero-alloc strings
func HashString(s string) uint64 {
	var hash uint64 = FnvOffset64

	if len(s) > 0 {
		_ = s[len(s)-1] // BCE

		for i := range len(s) {
			hash ^= uint64(s[i])
			hash *= FnvPrime64
		}
	}

	return hash
}

// HashBytesConcat computes the FNV-1a hash of multiple byte slices separated by a dot,
// without allocating a new concatenated slice on the heap.
func HashBytesConcat(a, sep, b []byte) uint64 {
	var hash uint64 = FnvOffset64

	if len(a) > 0 {
		_ = a[len(a)-1] // BCE

		for i := range a {
			hash ^= uint64(a[i])
			hash *= FnvPrime64
		}
	}

	if len(sep) > 0 {
		_ = sep[len(sep)-1] // BCE

		for i := range sep {
			hash ^= uint64(sep[i])
			hash *= FnvPrime64
		}
	}

	if len(b) > 0 {
		_ = b[len(b)-1] // BCE

		for i := range b {
			hash ^= uint64(b[i])
			hash *= FnvPrime64
		}
	}

	return hash
}
