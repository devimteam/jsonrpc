package json2

import (
	"encoding/json"
	"net/http"
	"reflect"
	"time"
	"io/ioutil"

	"github.com/l-vitaly/jsonrpc"
	"github.com/mitchellh/mapstructure"
)

// Version JSON RPC current version
var Version = "2.0"

// ----------------------------------------------------------------------------
// Request and Response
// ----------------------------------------------------------------------------

// serverRequest represents a JSON-RPC request received by the server.
type serverRequest struct {
	// JSON-RPC protocol.
	Version string `json:"jsonrpc"`

	// A String containing the name of the method to be invoked.
	Method string `json:"method"`

	// A Structured value to pass as arguments to the method.
	Params *json.RawMessage `json:"params"`

	// The request id. MUST be a string, number or null.
	// Our implementation will not do type checking for id.
	// It will be copied as it is.
	ID *json.RawMessage `json:"id"`
}

// serverResponse represents a JSON-RPC response returned by the server.
type serverResponse struct {
	// JSON-RPC protocol.
	Version string `json:"jsonrpc"`

	// The Object that was returned by the invoked method. This must be null
	// in case there was an error invoking the method.
	// As per spec the member will be omitted if there was an error.
	Result interface{} `json:"result,omitempty"`

	// An Error object if there was an error invoking the method. It must be
	// null if there was no error.
	// As per spec the member will be omitted if there was no error.
	Error *Error `json:"error,omitempty"`

	// This must be the same id as the request it is responding to.
	ID *json.RawMessage `json:"id"`
}

// ----------------------------------------------------------------------------
// Codec
// ----------------------------------------------------------------------------

// NewCustomCodec returns a new JSON Codec based on passed encoder selector.
func NewCustomCodec(encSel jsonrpc.EncoderSelector) *Codec {
	return &Codec{encSel: encSel}
}

// NewCodec returns a new JSON Codec.
func NewCodec() *Codec {
	return NewCustomCodec(jsonrpc.DefaultEncoderSelector)
}

// Codec creates a CodecRequest to process each request.
type Codec struct {
	encSel jsonrpc.EncoderSelector
}

// NewRequest returns a CodecRequest.
func (c *Codec) NewRequest(r *http.Request) jsonrpc.CodecRequest {
	return newCodecRequest(r, c.encSel.Select(r))
}

// ----------------------------------------------------------------------------
// CodecRequest
// ----------------------------------------------------------------------------

// newCodecRequest returns a new CodecRequest.
func newCodecRequest(r *http.Request, encoder jsonrpc.Encoder) jsonrpc.CodecRequest {
	defer r.Body.Close()

	// Decode the request body and check if RPC method is valid.
	body, _ := ioutil.ReadAll(r.Body)
	req := new(serverRequest)
	err := json.Unmarshal(body, req)



	if err != nil {
		err = &Error{
			Code:    ErrParse,
			Message: err.Error(),
		}
	} else if req.Version != Version {
		err = &Error{
			Code:    ErrInvalidRequest,
			Message: "jsonrpc must be " + Version,
		}
	}
	return &CodecRequest{request: req, err: err, encoder: encoder, body: body}
}

// CodecRequest decodes and encodes a single request.
type CodecRequest struct {
	request *serverRequest
	err     error
	encoder jsonrpc.Encoder
	body    []byte
}

func (c *CodecRequest) Body() []byte {
	return c.body
}

// Method returns the RPC method for the current request.
//
// The method uses a dotted notation as in "Service.Method".
func (c *CodecRequest) Method() (string, error) {
	if c.err == nil {
		return c.request.Method, nil
	}

	return "", c.err
}

func (c *CodecRequest) decoder(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	if t == reflect.TypeOf(time.Time{}) && f == reflect.TypeOf("") {
		return time.Parse(time.RFC3339, data.(string))
	}
	return data, nil
}

// ReadRequest fills the request object for the RPC method.
//
//
// ReadRequest parses request parameters in two supported forms in
// accordance with http://www.jsonrpc.org/specification#parameter_structures
//
// by-position: params MUST be an Array, containing the
// values in the Server expected order.
//
// by-name: params MUST be an Object, with member names
// that match the Server expected parameter names. The
// absence of expected names MAY result in an error being
// generated. The names MUST match exactly, including
// case, to the method's expected parameters.
func (c *CodecRequest) ReadRequest(args interface{}) error {
	if c.err == nil && c.request.Params != nil {
		var data map[string]interface{}
		if err := json.Unmarshal(*c.request.Params, &data); err != nil {
			c.err = &Error{
				Code:    ErrInvalidRequest,
				Message: err.Error(),
				Data:    c.request.Params,
			}
		} else {
			decoder, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
				DecodeHook:       c.decoder,
				TagName:          "ms",
				Result:           args,
				WeaklyTypedInput: false,
			})

			err := decoder.Decode(data)
			if err != nil {
				c.err = &Error{
					Code:    ErrBadParams,
					Message: err.Error(),
					Data:    c.request.Params,
				}
			}
		}
	}
	return c.err
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
func (c *CodecRequest) WriteResponse(w http.ResponseWriter, reply interface{}) {
	res := &serverResponse{
		Version: Version,
		Result:  reply,
		ID:      c.request.ID,
	}
	c.writeServerResponse(w, res)
}

// WriteError send error response.
func (c *CodecRequest) WriteError(w http.ResponseWriter, status int, err error) {
	jsonErr, ok := err.(*Error)

	if !ok {
		code := ErrInvalidRequest

		if err == jsonrpc.ErrMethodNotFound || err == jsonrpc.ErrServiceNotFound {
			code = ErrMethodNotFound
		}

		jsonErr = &Error{
			Code:    code,
			Message: err.Error(),
		}
	}

	res := &serverResponse{
		Version: Version,
		Error:   jsonErr,
		ID:      c.request.ID,
	}

	c.writeServerResponse(w, res)
}

func (c *CodecRequest) writeServerResponse(w http.ResponseWriter, res *serverResponse) {
	// Id is null for notifications and they don't have a response.
	if c.request.ID != nil || (res.Error != nil && (res.Error.Code == ErrParse || res.Error.Code == ErrInvalidRequest)) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		encoder := json.NewEncoder(c.encoder.Encode(w))
		err := encoder.Encode(res)
		// Not sure in which case will this happen. But seems harmless.
		if err != nil {
			jsonrpc.WriteError(w, 400, err.Error())
		}
	} else {
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.Header().Set("Json-Rpc", "notify")
    }
}

// EmptyResponse empty response
type EmptyResponse struct {
}
