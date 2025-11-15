package main

import (
	"fmt"
	"unsafe"

	"qmail-smtpd/sandbox/box/box"
)

type Message = box.Handle

//go:noinline
func process(msg Message) {
	switch msg := msg.Value().(type) {
	case *MessageA:
		fmt.Printf("msg: %v %+v\n", unsafe.Pointer(msg), msg)
	case *MessageB:
		fmt.Printf("msg: %v %+v\n", unsafe.Pointer(msg), msg)
	default:
		fmt.Printf("unknown message type %T", msg)
	}
	msg.Free()
}

type MessageA struct {
	Field string
}

type MessageB struct {
	Code int
}

func main() {
	boxA := box.Wrap(MessageA{Field: "Hello, world!"})
	msgA := boxA.Value()
	fmt.Printf("msg: %v %+v\n", unsafe.Pointer(msgA.(*MessageA)), msgA)
	process(boxA.Transfer())

	boxB := box.Wrap(MessageB{Code: 42})
	msgB := boxB.Value()
	fmt.Printf("msg: %v %+v\n", unsafe.Pointer(msgB.(*MessageB)), msgB)
	process(boxB.Transfer())
}

/* bash
go build -gcflags="-m" main.go 2>&1 | grep main.go | less
./main
*/
