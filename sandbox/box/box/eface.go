// == eface.go ==

package box

import "unsafe"

// eface — the raw interface{} structure, adapted to our *buffer.
//
// See:
// file://$GOROOT/src/runtime/runtime2.go
// https://github.com/golang/go/blob/master/src/runtime/runtime2.go
type eface struct {
	_type unsafe.Pointer
	data  *byte
}

func init() {
	if unsafe.Sizeof(any(nil)) != unsafe.Sizeof(eface{}) {
		panic("runtime interface{} layout mismatch")
	}

	type testStruct struct {
		_ int
	}

	ts := new(testStruct)
	var x any = ts
	if (*eface)(unsafe.Pointer(&x)).data != (*byte)(unsafe.Pointer(ts)) {
		panic("interface{} data field mismatch")
	}

	// TODO: check _type field
}
