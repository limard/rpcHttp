package main

import (
	"bissoft/rpcHttp"
	"bissoft/rpcHttp/bsonrpc"
	"bissoft/rpcHttp/jsonrpc2"
	"bissoft/rpcHttp/msgpackrpc"
	"fmt"
	"net"
	"net/http"
)

var host = "127.0.0.1:8199"

func main() {
	s := rpcHttp.NewServer()
	s.RegisterCodec(jsonrpc2.NewCodec(), jsonrpc2.ContentType)
	s.RegisterCodec(bsonrpc.NewCodec(), bsonrpc.ContentType)
	s.RegisterCodec(msgpackrpc.NewCodec(), msgpackrpc.ContentType)

	s.RegisterService(new(Service), "")
	http.Handle("/", s)

	l, err := net.Listen(`tcp`, host)
	if err != nil {
		panic(err)
	}

	fmt.Println(`Port:`, l.Addr().String())
	http.Serve(l, nil)
	if err != nil {
		panic(err)
	}
}

type Service struct {
}

type REQ struct {
	Message string
}

type REP struct {
	Message string
}

func (t *Service) Name(req *http.Request, request *REQ, reply *REP) error {
	fmt.Println("Request:", *request)
	reply.Message = "reply message"

	return nil
}
