// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpcHttp

import (
	"fmt"
	"log"
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
	WriteErrorResponse(w http.ResponseWriter, status int, err error, data interface{})
}

const (
	E_PARSE       = -32700
	E_INVALID_REQ = -32600
	E_NO_METHOD   = -32601
	E_BAD_PARAMS  = -32602
	E_INTERNAL    = -32603
	E_SERVER      = -32000
)

// ----------------------------------------------------------------------------
// Server
// ----------------------------------------------------------------------------

// NewServer returns a new RPC server.
func NewServer() *Server {
	return &Server{
		codecs:         make(map[string]Codec),
		services:       new(serviceMap),
		postMethodOnly: true,
	}
}

// Server serves registered RPC services using registered codecs.
type Server struct {
	codecs           map[string]Codec
	services         *serviceMap
	methodIgnoreCase bool
	postMethodOnly   bool
}

func (s *Server) SetPostMethodOnly(postMethodOnly bool) {
	s.postMethodOnly = postMethodOnly
}

func (s *Server) SetMethodIgnoreCase(ignoreCase bool) {
	s.methodIgnoreCase = ignoreCase
	s.services.methodIgnoreCase = ignoreCase
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

// EnumMethodInfo to get information of each method
func (s *Server) EnumMethodInfo() []string {
	return s.services.enumMethodInfo()
}

func (s *Server) EnumMethod() []string {
	return s.services.enumMethod()
}

func (s *Server) MethodPage(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Method:\n"))
	names := s.EnumMethodInfo()
	count := len(names)
	for i := 0; i < count; i++ {
		w.Write([]byte(names[i] + "\n"))
	}
}

// ServeHTTP
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if s.postMethodOnly {
		if r.Method != "POST" {
			log.Printf("POST method required, Current: %s", r.Method)
			WriteError(w, 405, "rpc: POST method required, received "+r.Method)
			return
		}
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
		log.Printf("unrecognized Content-Type(%s)", contentType)
		WriteError(w, 415, "rpc: unrecognized Content-Type: "+contentType)
		return
	}
	// Create a new codec request.
	codecReq := codec.NewRequest(r)
	// Get service method to be called.
	method, errMethod := codecReq.Method()
	if errMethod != nil {
		log.Println("errMethod", errMethod)
		codecReq.WriteErrorResponse(w, 400, errMethod, nil)
		return
	}
	serviceSpec, methodSpec, errGet := s.services.get(method)
	if errGet != nil {
		log.Println("errGet", errGet)
		codecReq.WriteErrorResponse(w, 400, errGet, nil)
		return
	}
	// Decode the args.
	args := reflect.New(methodSpec.argsType)
	if errRead := codecReq.ReadRequest(args.Interface()); errRead != nil {
		log.Println("errRead", errRead)
		codecReq.WriteErrorResponse(w, 400, errRead, nil)
		return
	}
	// Call the service method.
	reply := reflect.New(methodSpec.replyType)
	methodSpec.counter++

	params := make([]reflect.Value, 0)
	params = append(params, serviceSpec.rcvr)
	if methodSpec.hasHttpReq {
		params = append(params, reflect.ValueOf(r))
		if methodSpec.hasHttpRes {
			params = append(params, reflect.ValueOf(w))
		}
	}
	params = append(params, args)
	params = append(params, reply)

	resValue := methodSpec.method.Func.Call(params)

	// Cast the result to error if needed.
	errCode := E_SERVER
	var errResult error
	var errData interface{}
	if len(resValue) == 1 {
		errInter := resValue[0].Interface()
		if errInter != nil {
			errResult = errInter.(error)
		}
	} else if len(resValue) == 2 {
		errCode = int(resValue[0].Int())
		errInter := resValue[1].Interface()
		if errInter != nil {
			errResult = errInter.(error)
		}
	} else {
		errCode = int(resValue[0].Int())
		errInter := resValue[1].Interface()
		if errInter != nil {
			errResult = errInter.(error)
		}
		errData = resValue[2].Interface()
	}

	// Prevents Internet Explorer from MIME-sniffing a response away
	// from the declared content-type
	w.Header().Set("x-content-type-options", "nosniff")
	// Encode the response.
	if errResult == nil {
		// success response
		codecReq.WriteResponse(w, reply.Interface())
		return
	}
	// error response
	log.Println("write err:", errResult)
	codecReq.WriteErrorResponse(w, errCode, errResult, errData)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, msg)
}
