package main

import (
	"fmt"
	"github.com/Limard/rpcHttp/bsonrpc"
	"gopkg.in/mgo.v2/bson"
)

func main() {
	var e error
	type REQ struct {
		Message string
	}

	type REP struct {
		Message string
	}

	req := REQ{"asdfghjkl"}
	reply := REP{}

	_, e = bson.Marshal(req)
	if e != nil {
		fmt.Println("bson.Marshal", e)
		return
	}

	e = bsonrpc.Call("http://127.0.0.1:8199", "Service.Name", &req, &reply)
	if e != nil {
		fmt.Println(e)
		return
	}
	fmt.Println(reply)
}
