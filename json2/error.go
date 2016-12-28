package json2

import (
	"errors"
)

// ErrorCode JSON RPC error code type
type ErrorCode int

const (
	// ErrParse Invalid JSON was received by the server.
	ErrParse ErrorCode = -32700
	// ErrInvalidRequest The JSON sent is not a valid Request object.
	ErrInvalidRequest ErrorCode = -32600
	// ErrMethodNotFound The method does not exist / is not available.
	ErrMethodNotFound ErrorCode = -32601
	// ErrBadParams Invalid method parameter(s).
	ErrBadParams ErrorCode = -32602
	// ErrInternal Internal JSON-RPC error.
	ErrInternal ErrorCode = -32603
	// ErrServer Reserved for implementation-defined server-errors.
	ErrServer ErrorCode = -32000
)

// ErrNullResult result is null
var ErrNullResult = errors.New("result is null")

// Error JSON RPC error structure
type Error struct {
	// A Number that indicates the error type that occurred.
	Code ErrorCode `json:"code"`

	// A String providing a short description of the error.
	// The message SHOULD be limited to a concise single sentence.
	Message string `json:"message"`

	// A Primitive or Structured value that contains additional information about the error.
	Data interface{} `json:"data,omitempty"`
}

// NewError create a new error
func NewError(code ErrorCode, message interface{}) *Error {
	strErr, ok := message.(string)

	if !ok {
		err, ok := message.(error)

		if ok {
			strErr = err.Error()
		}
	}

	return &Error{Code: code, Message: strErr}
}

func (e *Error) Error() string {
	return e.Message
}
