// == tesutils_test.go ==

package box

import (
	"errors"
	"fmt"
	"sync"
)

// Types of messages for our protocol
type MessageType int

const (
	MessageTypeRequest MessageType = iota + 1
	MessageTypeResponse
	MessageTypeHeartbeat
	MessageTypeError
)

var ErrUnknownMsgType = errors.New("unknown message type")

// Message structures (all less than bufferSize)
type Request struct {
	ID      uint64
	Method  string
	Path    string
	Payload [64]byte
}

type Response struct {
	ID      uint64
	Status  int
	Payload [96]byte
}

type Heartbeat struct {
	NodeID  string
	Counter uint64
}

type Error struct {
	Code    int
	Message string
	Details [32]byte
}

//go:noinline
func nativeProducer(i int) any {
	messageID := uint64(i)
	var msg any
	switch i % 4 {
	case 0:
		msg = &Request{
			ID:     messageID,
			Method: "GET",
			Path:   "/api/v1/data",
		}
	case 1:
		msg = &Response{
			ID:     messageID,
			Status: 200,
		}
	case 2:
		msg = &Heartbeat{
			NodeID:  "node-1",
			Counter: messageID,
		}
	case 3:
		msg = &Error{
			Code:    500,
			Message: "internal error",
		}
	}
	return msg
}

//go:noinline
func nativeConsumer(msg any) error {
	switch msg.(type) {
	case *Request, *Response, *Heartbeat, *Error:
		return nil
	default:
		return fmt.Errorf("%v: %T", ErrUnknownMsgType, msg)
	}
}

type Pool[T any] struct{ sync.Pool }

func NewPool[T any]() *Pool[T] {
	return &Pool[T]{
		Pool: sync.Pool{
			New: func() any { return new(T) },
		},
	}
}

type DedicatedPools struct {
	reqPool  *Pool[Request]
	respPool *Pool[Response]
	hbPool   *Pool[Heartbeat]
	errPool  *Pool[Error]
}

func NewDedicatedPools() *DedicatedPools {
	return &DedicatedPools{
		reqPool:  NewPool[Request](),
		respPool: NewPool[Response](),
		hbPool:   NewPool[Heartbeat](),
		errPool:  NewPool[Error](),
	}
}

//go:noinline
func poolsProducer(i int, dp *DedicatedPools) any {
	messageID := uint64(i) + 1
	var msg any
	switch i % 4 {
	case 0:
		req := dp.reqPool.Get().(*Request)
		req.ID = messageID
		req.Method = "GET"
		req.Path = "/api/v1/data"
		msg = req
	case 1:
		resp := dp.respPool.Get().(*Response)
		resp.ID = messageID
		resp.Status = 200
		msg = resp
	case 2:
		hb := dp.hbPool.Get().(*Heartbeat)
		hb.NodeID = "node-1"
		hb.Counter = messageID
		msg = hb
	case 3:
		err := dp.errPool.Get().(*Error)
		err.Code = 500
		err.Message = "internal error"
		msg = err
	}
	return msg
}

//go:noinline
func poolsConsumer(msg any, dp *DedicatedPools) error {
	switch msg := msg.(type) {
	case *Request:
		*msg = Request{}
		dp.reqPool.Put(msg)
		return nil
	case *Response:
		*msg = Response{}
		dp.respPool.Put(msg)
		return nil
	case *Heartbeat:
		*msg = Heartbeat{}
		dp.hbPool.Put(msg)
		return nil
	case *Error:
		*msg = Error{}
		dp.errPool.Put(msg)
		return nil
	default:
		return fmt.Errorf("%v: %T", ErrUnknownMsgType, msg)
	}
}

//go:noinline
func boxProducer(i int) Handle {
	messageID := uint64(i)
	switch i % 4 {
	case 0:
		return Wrap(Request{
			ID:     messageID,
			Method: "GET",
			Path:   "/api/v1/data",
		})
	case 1:
		return Wrap(Response{
			ID:     messageID,
			Status: 200,
		})
	case 2:
		return Wrap(Heartbeat{
			NodeID:  "node-1",
			Counter: messageID,
		})
	case 3:
		return Wrap(Error{
			Code:    500,
			Message: "internal error",
		})
	}
	return Handle{} // stub
}

//go:noinline
func boxConsumer(h Handle) error {
	switch h.Value().(type) {
	case *Request, *Response, *Heartbeat, *Error:
		h.Free()
		return nil
	default:
		return fmt.Errorf("%v: %T", ErrUnknownMsgType, h.Value())
	}
}
