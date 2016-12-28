package jsonrpc

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
    "github.com/pkg/errors"
)

var (
	// Precompute the reflect.Type of error and http.Request
	typeOfError = reflect.TypeOf((*error)(nil)).Elem()
	typeOfRequest = reflect.TypeOf((*http.Request)(nil)).Elem()
)

var (
    ErrRequestIllFormed = errors.New("service/method request ill-formed")
    ErrServiceNotFound = errors.New("can't find service")
    ErrMethodNotFound = errors.New("rpc: can't find method")
)

// ----------------------------------------------------------------------------
// service
// ----------------------------------------------------------------------------

type service struct {
	name     string                    // name of service
	rcvr     reflect.Value             // receiver of methods for the service
	rcvrType reflect.Type              // type of the receiver
	methods  map[string]*serviceMethod // registered methods
}

type serviceMethod struct {
	method    reflect.Method // receiver method
	argsType  []reflect.Type // type of the request argument
	replyType reflect.Type   // type of the response argument
}

// ----------------------------------------------------------------------------
// serviceMap
// ----------------------------------------------------------------------------

// serviceMap is a registry for services.
type serviceMap struct {
	mutex    sync.Mutex
	services map[string]*service
}

// register adds a new service using reflection to extract its methods.
func (m *serviceMap) register(rcvr interface{}, name string) error {
	s := &service{
		name:     name,
		rcvr:     reflect.ValueOf(rcvr),
		rcvrType: reflect.TypeOf(rcvr),
		methods:  make(map[string]*serviceMethod),
	}

	if name == "" {
		s.name = reflect.Indirect(s.rcvr).Type().Name()
		if !isExported(s.name) {
			return fmt.Errorf("rpc: type %q is not exported", s.name)
		}
	}

	if s.name == "" {
		return fmt.Errorf("rpc: no service name for type %q", s.rcvrType.String())
	}

	for i := 0; i < s.rcvrType.NumMethod(); i++ {
		method := s.rcvrType.Method(i)
		mtype := method.Type

		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}

		var args []reflect.Type

		numIn := mtype.NumIn()

		for i := 1; i < numIn; i++ {
			arg := mtype.In(i)

			if arg.Kind() != reflect.Ptr || !isExportedOrBuiltin(arg) {
				continue
			}

			args = append(args, arg.Elem())
		}

		if numIn-1 != len(args) {
			continue
		}

		// Method needs two out: mixed, error.
		if mtype.NumOut() != 2 {
			continue
		}

		if returnType := mtype.Out(1); returnType != typeOfError {
			continue
		}

		s.methods[method.Name] = &serviceMethod{
			method:   method,
			argsType: args,
		}
	}

	if len(s.methods) == 0 {
		return fmt.Errorf("rpc: %q has no exported methods of suitable type", s.name)
	}

	m.mutex.Lock()

	defer m.mutex.Unlock()

	if m.services == nil {
		m.services = make(map[string]*service)
	} else if _, ok := m.services[s.name]; ok {
		return fmt.Errorf("rpc: service already defined: %q", s.name)
	}

	m.services[s.name] = s

	return nil
}

// get returns a registered service given a method name.
//
// The method name uses a dotted notation as in "Service.Method".
func (m *serviceMap) get(method string) (*service, *serviceMethod, error) {
	parts := strings.Split(method, ".")
	if len(parts) != 2 {
		return nil, nil, ErrRequestIllFormed
	}

	m.mutex.Lock()
	service := m.services[parts[0]]
	m.mutex.Unlock()

	if service == nil {
		return nil, nil, ErrServiceNotFound
	}

	serviceMethod := service.methods[parts[1]]
	if serviceMethod == nil {
		return nil, nil, ErrMethodNotFound
	}

	return service, serviceMethod, nil
}

// isExported returns true of a string is an exported (upper case) name.
func isExported(name string) bool {
	rune, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(rune)
}

// isExportedOrBuiltin returns true if a type is exported or a builtin.
func isExportedOrBuiltin(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return isExported(t.Name()) || t.PkgPath() == ""
}
