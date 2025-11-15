// == box.go ==

// Package box provides memory pooling with explicit ownership semantics.
//
// It is designed for high-performance scenarios involving small, short-lived,
// heterogeneous messages — such as event buses, actor systems, or protocol dispatchers —
// where minimizing garbage collection pressure is critical.
//
// Operation Modes:
//
// The package supports two build modes for development and production:
//   - Reckless mode (default): Maximum performance with minimal checks
//   - Paranoid mode (-tags paranoid): Validation with panic on contract violations
//
// Use paranoid mode during development to detect ownership errors, then switch to
// default mode for production deployment after thorough testing.
//
// Key Features:
//
//   - Zero allocations for values fitting in BufferSize (128 bytes)
//   - Explicit ownership tracking with transferable handles
//   - Type-safe value access with compile-time checks
//   - Thread-safe for concurrent use (with proper ownership contract)
//
// Basic Usage:
//
//	// Wrap a value for managed storage
//	h := box.Wrap(MyMessage{Data: "hello"})
//
//	// Transfer ownership — choose one of these patterns:
//
//	// 1. Send via channel
//	channel <- h.Transfer()
//
//	// 2. Pass to function
//	process(h.Transfer())
//
//	// 3. Return from function
//	return h.Transfer()
//
//	// --- On the receiving side ---
//
//	// Access by pointer (temporary! do not save/share)
//	if pointer, ok := h.Value().(*MyMessage); ok {
//		// ...
//	}
//	h.Free()
//
//	// OR access by value (auto-free on success)
//	if value, ok := box.Unwrap[MyMessage](h); ok {
//		// ...
//	} else {
//		h.Free()
//	}
//
// Performance Characteristics:
//
//   - Near-zero overhead over sync.Pool (primary cost is memory management)
//   - Paranoid mode: ~35ns/op (contract violation detection)
//   - Reckless mode: ~20ns/op (minimal checks, maximum speed)
//   - Zero allocations per operation in both modes
//
// The core value is enforcement of a clear ownership contract through panics.
//
// OWNERSHIP CONTRACT:
//
// WARNING: In reckless mode, NO runtime checks are performed.
// Safety depends ENTIRELY on correct usage of the ownership contract.
// Violations result in undefined behavior (memory corruption, silent data races, etc.).
//
// 1. EXCLUSIVE OWNERSHIP
//   - Each Handle has exactly one owner at any time
//   - Ownership can be transferred but not shared
//   - After Transfer() or Free(), all Handle copies become invalid
//
// 2. LIFETIME MANAGEMENT
//   - The owner must call Transfer() or Free() exactly once
//   - Values must not be used after Transfer() or Free()
//   - Clone() creates independent ownership requiring separate management
//
// 3. TRANSFER PROTOCOL
//   - Use Transfer() for explicit ownership handover
//   - After transfer, the original owner must not use the Handle
//   - Receiving party becomes the new exclusive owner
//
// Contract violations panic in paranoid mode and cause undefined behavior in production.
// The package helps detect violations but cannot prevent them - safety comes from correct usage.
package box

import (
	"sync"
	"unsafe"
)

// NOTE:
//
//   - 'paranoid' is a build-time constant (defined in paranoid.go or reckless.go).
//     The compiler eliminates all dead branches — zero overhead in production.
//
//   - getBuf() and invalidateHandles() are implemented per build mode
//     in paranoid.go (with validation) and reckless.go (minimal cost, no ownership tracking).

// BufferSize is the maximum size (in bytes) of a value passed to Wrap().
// Exceeding it panics in paranoid mode, causes UB in reckless mode.
const BufferSize = 128

type buffer struct {
	ver  bufVer
	data [BufferSize]byte
}

var pool = sync.Pool{
	New: func() any {
		b := new(buffer)
		return b
	},
}

// Possible panic reasons in paranoid mode.
// These are not errors, but panic types for detection during development.
const (
	ValueTooLarge    = "value too large"            // From Wrap
	InvalidHandle    = "invalid handle operation"   // From Value, Clone, Free, Transfer
	ConcurrentAccess = "concurrent access detected" // From Free, Transfer
)

// Handle is an exclusive owner of a boxed value.
type Handle struct {
	value  any
	len    int
	bufVer bufVer
}

// Wrap boxes a value of type T into a new Handle.
func Wrap[T any](value T) Handle {
	// TODO: figure out how to check this at compilation time.
	if paranoid && unsafe.Sizeof(value) > BufferSize {
		panic("box.Wrap: " + ValueTooLarge)
	}

	buf := pool.Get().(*buffer)
	data := (*T)(unsafe.Pointer(&buf.data[0]))
	*data = value

	if paranoid {
		return Handle{
			value:  data,
			len:    int(unsafe.Sizeof(value)),
			bufVer: buf.ver,
		}
	}

	return Handle{
		value: data,
		len:   int(unsafe.Sizeof(value)),
	}
}

// EXPERIMENTAL: may be removed or changed.
//
// Unwrap extracts the value if it is of type T.
// If type matches: returns the value and true — Handle is automatically freed.
// If type does not match: returns zero value and false — Handle is NOT freed.
func Unwrap[T any](h Handle) (T, bool) {
	if p, ok := h.value.(*T); ok {
		v := *p
		h.Free()
		return v, true
	}

	var zero T
	return zero, false
}

// EXPERIMENTAL: may be removed or changed.
//
// MustUnwrap (unsafe Unwrap) returns wrapped value and frees Handle.
// Panics if the underlying value is not of type T.
func MustUnwrap[T any](h Handle) (v T) {
	v = *h.value.(*T)
	h.Free()
	return v
}

// Value returns the boxed value as interface{}.
func (h Handle) Value() any {
	if paranoid {
		if buf := h.getBuf(); buf == nil {
			panic("box.Handle.Value: " + InvalidHandle)
		}
	}
	return h.value
}

// Clone creates an independent copy of the Handle and its underlying value.
// The cloned Handle must be managed separately (Free or Transfer).
func (h Handle) Clone() Handle {
	buf := h.getBuf()

	if paranoid {
		if buf == nil {
			panic("box.Handle.Clone: " + InvalidHandle)
		}
	}

	buf2 := pool.Get().(*buffer)

	src := unsafe.Slice((*byte)(&buf.data[0]), h.len)
	dst := unsafe.Slice((*byte)(&buf2.data[0]), h.len)
	copy(dst, src)

	var h2 Handle
	ef := (*eface)(unsafe.Pointer(&h.value))
	ef2 := (*eface)(unsafe.Pointer(&h2.value))
	ef2._type = ef._type
	ef2.data = &buf2.data[0]

	h2.len = h.len
	h2.bufVer = buf2.ver

	return h2
}

// Free releases the underlying buffer back to the pool.
func (h Handle) Free() {
	buf := h.getBuf()

	if paranoid {
		if buf == nil {
			panic("box.Handle.Free: " + InvalidHandle)
		}
		if !h.invalidateHandles(buf) {
			panic("box.Handle.Free: " + ConcurrentAccess)
		}
	}

	clear(buf.data[:h.len])
	pool.Put(buf)
}

// Transfer transfers ownership on value.
func (h Handle) Transfer() Handle {
	if paranoid {
		buf := h.getBuf()
		if buf == nil {
			panic("box.Handle.Transfer: " + InvalidHandle)
		}
		if !h.invalidateHandles(buf) {
			panic("box.Handle.Transfer: " + ConcurrentAccess)
		}
		return Handle{
			value:  h.value,
			len:    h.len,
			bufVer: buf.ver,
		}
	}

	return h
}
