// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpcHttp

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var (
	// Precompute the reflect.Type of error and http.Request
	typeOfError          = reflect.TypeOf((*error)(nil)).Elem()
	typeOfRequest        = reflect.TypeOf((*http.Request)(nil)).Elem()
	typeOfResponseWriter = reflect.TypeOf((*http.ResponseWriter)(nil)).Elem()
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
	method     reflect.Method // receiver method
	hasHttpReq bool
	hasHttpRes bool
	argsType   reflect.Type // type of the request argument
	replyType  reflect.Type // type of the response argument
	counter    int          // used to record the number of calls
}

// ----------------------------------------------------------------------------
// serviceMap
// ----------------------------------------------------------------------------

// serviceMap is a registry for services.
type serviceMap struct {
	mutex            sync.Mutex
	services         map[string]*service
	methodIgnoreCase bool
}

// register adds a new service using reflection to extract its methods.
func (m *serviceMap) register(rcvr interface{}, name string) error {
	// Setup service.
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
		return fmt.Errorf("rpc: no service name for type %q",
			s.rcvrType.String())
	}
	// Setup methods.
	for i := 0; i < s.rcvrType.NumMethod(); i++ {
		method := s.rcvrType.Method(i)
		mtype := method.Type
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		// or Method needs four ins: receiver, *http.Request, *args, *reply.
		// or Method needs five ins: receiver, *http.Request, *http.Response, *args, *reply.

		var hasHttpReq bool
		var hasHttpRes bool
		if mtype.NumIn() == 3 {
			hasHttpReq = false
			hasHttpRes = false
		} else if mtype.NumIn() == 4 {
			hasHttpReq = true
			hasHttpRes = false
		} else if mtype.NumIn() == 5 {
			hasHttpReq = true
			hasHttpRes = true
		} else {
			continue
		}

		argIndex := 1

		if hasHttpReq {
			// First argument must be a pointer and must be http.Request.
			reqType := mtype.In(argIndex)
			if reqType.Kind() != reflect.Ptr || reqType.Elem() != typeOfRequest {
				continue
			}
			argIndex++
		}

		if hasHttpRes {
			// First argument must be a pointer and must be http.Request.
			reqType := mtype.In(argIndex)
			//if reqType.Kind() != reflect.Ptr || reqType.Elem() != typeOfResponseWriter {
			if reqType != typeOfResponseWriter {
				continue
			}
			argIndex++
		}
		// Second argument must be a pointer and must be exported.
		args := mtype.In(argIndex)
		if args.Kind() != reflect.Ptr || !isExportedOrBuiltin(args) {
			continue
		}
		argIndex++
		// Third argument must be a pointer and must be exported.
		reply := mtype.In(argIndex)
		if reply.Kind() != reflect.Ptr || !isExportedOrBuiltin(reply) {
			continue
		}
		argIndex++

		// Method needs
		// one out: 		error(message).
		// or two out: 		errorNumber(int) error(message).
		// or three out: 	errorNumber(int) error(message) data(interface).
		if mtype.NumOut() == 1 {
			if returnType := mtype.Out(0); returnType != typeOfError {
				continue
			}
		} else if mtype.NumOut() == 2 {
			if returnType := mtype.Out(0); returnType.Kind() != reflect.Int {
				continue
			}
			if returnType := mtype.Out(1); returnType != typeOfError {
				continue
			}
		} else if mtype.NumOut() == 3 {
			if returnType := mtype.Out(0); returnType.Kind() != reflect.Int {
				continue
			}
			if returnType := mtype.Out(1); returnType != typeOfError {
				continue
			}
			// no.2 unlimited format
		} else {
			continue
		}

		if m.methodIgnoreCase {
			s.methods[strings.ToLower(method.Name)] = &serviceMethod{
				method:     method,
				argsType:   args.Elem(),
				replyType:  reply.Elem(),
				hasHttpRes: hasHttpRes,
				hasHttpReq: hasHttpReq,
			}
		} else {
			s.methods[method.Name] = &serviceMethod{
				method:     method,
				argsType:   args.Elem(),
				replyType:  reply.Elem(),
				hasHttpRes: hasHttpRes,
				hasHttpReq: hasHttpReq,
			}
		}

	}
	if len(s.methods) == 0 {
		return fmt.Errorf("rpc: %q has no exported methods of suitable type",
			s.name)
	}
	// Add to the map.
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.services == nil {
		m.services = make(map[string]*service)
	} else if _, ok := m.services[s.name]; ok {
		return fmt.Errorf("rpc: service already defined: %q", s.name)
	}
	if m.methodIgnoreCase {
		m.services[strings.ToLower(s.name)] = s
	} else {
		m.services[s.name] = s
	}
	return nil
}

// get returns a registered service given a method name.
//
// The method name uses a dotted notation as in "Service.Method".
func (m *serviceMap) get(method string) (*service, *serviceMethod, error) {
	methodForDisplay := method
	if m.methodIgnoreCase {
		method = strings.ToLower(method)
	}
	parts := strings.Split(method, ".")
	if len(parts) != 2 {
		// for method name not include period char(.)
		if len(parts) == 1 {
			for _, service := range m.services {
				serviceMethod := service.methods[parts[0]]
				if serviceMethod == nil {
					continue
				}
				return service, serviceMethod, nil
			}
		}

		err := fmt.Errorf("rpc: service/method request ill-formed: %q", method)
		return nil, nil, err
	}
	m.mutex.Lock()
	service := m.services[parts[0]]
	m.mutex.Unlock()
	if service == nil {
		err := fmt.Errorf("rpc: can't find service %q", methodForDisplay)
		return nil, nil, err
	}
	serviceMethod := service.methods[parts[1]]
	if serviceMethod == nil {
		err := fmt.Errorf("rpc: can't find method %q", methodForDisplay)
		return nil, nil, err
	}
	return service, serviceMethod, nil
}

func (m *serviceMap) enumMethodInfo() (methodNames []string) {
	for _, s := range m.services {
		for mn, mv := range s.methods {
			methodNames = append(methodNames, fmt.Sprintf(`%v.%v(calls:%v)`, s.name, mn, mv.counter))
		}
	}
	return methodNames
}

func (m *serviceMap) enumMethod() (methodNames []string) {
	for _, s := range m.services {
		for _, mv := range s.methods {
			methodNames = append(methodNames, fmt.Sprintf(`%s.%s`, s.name, mv.method.Name))
		}
	}
	return methodNames
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
