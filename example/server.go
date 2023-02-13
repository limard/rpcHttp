//go:build none
// +build none

package main

import (
	"fmt"
	"net"
	"net/http"

	"github.com/Limard/rpcHttp"
	"github.com/Limard/rpcHttp/bsonrpc"
	"github.com/Limard/rpcHttp/jsonrpc2"
	"github.com/Limard/rpcHttp/msgpackrpc"
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
