package jsonrpc

import (
    "context"
    "fmt"
    "net/http"
    "reflect"
    "strings"
)

// ----------------------------------------------------------------------------
// Codec
// ----------------------------------------------------------------------------

// Codec creates a CodecRequest to process each request.
type Codec interface {
    NewRequest(*http.Request) CodecRequest
}

// CodecRequest decodes a request and encodes a response using a specific
// serialization scheme.
type CodecRequest interface {
    // Reads the request and returns the RPC method name.
    Method() (string, error)
    // Reads the request filling the RPC method args.
    ReadRequest(interface{}) error
    // Writes the response using the RPC method reply.
    WriteResponse(http.ResponseWriter, interface{})
    // Writes an error produced by the server.
    WriteError(w http.ResponseWriter, status int, err error)
    // Get raw body
    Body() []byte
}

// ----------------------------------------------------------------------------
// Server
// ----------------------------------------------------------------------------

type ServerBeforeFunc func(ctx context.Context, method string, header http.Header, req CodecRequest) context.Context

// Server serves registered RPC services using registered codecs.
type Server struct {
    codecs   map[string]Codec
    services *serviceMap
    before   []ServerBeforeFunc
}

type ServerOption func(*Server)

func ServerBefore(before ServerBeforeFunc) ServerOption {
    return func(s *Server) { s.before = append(s.before, before) }
}

// NewServer returns a new RPC server.
func NewServer(options ...ServerOption) *Server {
    s := &Server{
        codecs:   make(map[string]Codec),
        services: new(serviceMap),
    }
    for _, option := range options {
        option(s)
    }
    return s
}

// RegisterCodec adds a new codec to the server.
//
// Codecs are defined to process a given serialization scheme, e.g., JSON or
// XML. A codec is chosen based on the "Content-Type" header from the request,
// excluding the charset definition.
func (s *Server) RegisterCodec(codec Codec, contentType string) {
    s.codecs[strings.ToLower(contentType)] = codec
}

// RegisterService adds a new service to the server.
//
// The name parameter is optional: if empty it will be inferred from
// the receiver type name.
//
// Methods from the receiver will be extracted if these rules are satisfied:
//
//    - The receiver is exported (begins with an upper case letter) or local
//      (defined in the package registering the service).
//    - The method name is exported.
//    - The method has three arguments: *http.Request, *args, *reply.
//    - All three arguments are pointers.
//    - The second and third arguments are exported or local.
//    - The method has return type error.
//
// All other methods are ignored.
func (s *Server) RegisterService(receiver interface{}, name string) error {
    return s.services.register(receiver, name)
}

// HasMethod returns true if the given method is registered.
//
// The method uses a dotted notation as in "Service.Method".
func (s *Server) HasMethod(method string) bool {
    if _, _, err := s.services.get(method); err == nil {
        return true
    }
    return false
}

// ServeHTTP
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    if r.Method != "POST" {
        WriteError(w, 405, "rpc: POST method required, received "+r.Method)
        return
    }
    contentType := r.Header.Get("Content-Type")
    idx := strings.Index(contentType, ";")

    if idx != -1 {
        contentType = contentType[:idx]
    }

    var codec Codec

    if contentType == "" && len(s.codecs) == 1 {
        // If Content-Type is not set and only one codec has been registered,
        // then default to that codec.
        for _, c := range s.codecs {
            codec = c
        }
    } else if codec = s.codecs[strings.ToLower(contentType)]; codec == nil {
        WriteError(w, 415, "rpc: unrecognized Content-Type: "+contentType)
        return
    }

    // Create a new codec request.
    codecReq := codec.NewRequest(r)

    // Get service method to be called.
    method, errMethod := codecReq.Method()
    if errMethod != nil {
        codecReq.WriteError(w, 400, errMethod)
        return
    }

    for _, before := range s.before {
        ctx = before(ctx, method, r.Header, codecReq)
    }

    serviceSpec, methodSpec, errGet := s.services.get(method)
    if errGet != nil {
        codecReq.WriteError(w, 400, errGet)
        return
    }
    refValue := []reflect.Value{serviceSpec.rcvr}
    // Decode the args.
    if len(methodSpec.argsType) > 0 {
        for i := 0; i < len(methodSpec.argsType); i++ {
            arg := reflect.New(methodSpec.argsType[i])
            if methodSpec.argsType[i] != typeOfContext {
                if errRead := codecReq.ReadRequest(arg.Interface()); errRead != nil {
                    codecReq.WriteError(w, 400, errRead)
                    return
                }
            } else {
                arg = reflect.ValueOf(ctx)
            }
            refValue = append(refValue, arg)
        }
    }

    retValues := methodSpec.method.Func.Call(refValue)

    // Cast the result to error if needed.
    var errResult error
    errInter := retValues[1].Interface()
    if errInter != nil {
        errResult = errInter.(error)
    }

    // Prevents Internet Explorer from MIME-sniffing a response away
    // from the declared content-type
    w.Header().Set("x-content-type-options", "nosniff")

    // Encode the response.
    if errResult == nil {
        valRet := retValues[0].Interface()
        codecReq.WriteResponse(w, valRet)
    } else {
        codecReq.WriteError(w, 400, errResult)
    }
}

// WriteError send error to client
func WriteError(w http.ResponseWriter, status int, msg string) {
    w.WriteHeader(status)
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    fmt.Fprint(w, msg)
}
