package bsonrpc

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
)

var ContentType = `application/bson`

// clientRequest represents a JSON-RPC request sent by a client.
type clientRequest struct {
	Version string      `bson:"msgpackrpc"`
	Method  string      `bson:"method"`
	Params  interface{} `bson:"params"`
	Id      interface{} `bson:"id"`
}

// clientResponse represents a JSON-RPC response returned to a client.
type clientResponse struct {
	Id      interface{} `bson:"id"`
	Version string      `bson:"msgpackrpc"`
	Result  interface{} `bson:"result"`
	Error   interface{} `bson:"error"`
}

func encodeClientRequest(method string, args interface{}) ([]byte, error) {
	c := &clientRequest{
		Version: "1.0",
		Method:  method,
		Params:  args,
		Id:      uint64(rand.Int63()),
	}
	return bson.Marshal(c)
}

func decodeClientResponse(r io.Reader, reply interface{}) (e error) {
	var c clientResponse
	buf, e := ioutil.ReadAll(r)
	if e != nil {
		return e
	}
	e = bson.Unmarshal(buf, &c)
	if e != nil {
		return e
	}

	// Error
	if c.Error != nil {
		replyError := &Error{}
		tempBuf, _ := bson.Marshal(c.Error)
		if err := bson.Unmarshal(tempBuf, replyError); err != nil {
			return &Error{
				Code:    E_PARSE,
				Message: string(tempBuf),
			}
		}
		return replyError
	}

	// Result
	if c.Result == nil {
		return &Error{
			Code:    E_BAD_PARAMS,
			Message: "result is null",
		}
	}
	tempBuf, _ := bson.Marshal(c.Result)
	if err := bson.Unmarshal(tempBuf, reply); err != nil {
		return &Error{
			Code:    E_PARSE,
			Message: err.Error(),
		}
	}

	return nil
}

func Call(url string, method string, request interface{}, reply interface{}) (e error) {
	jsonReqBuf, err := encodeClientRequest(method, request)
	if err != nil {
		return &Error{
			Code:    E_INVALID_REQ,
			Message: err.Error()}
	}

	jsonReqBufR := bytes.NewReader(jsonReqBuf)
	rsp, err := http.Post(url, ContentType, jsonReqBufR)
	if err != nil {
		return &Error{
			Code:    E_SERVER,
			Message: err.Error()}
	}
	defer rsp.Body.Close()

	return decodeClientResponse(rsp.Body, reply)
}

func ConvertError(err error) (replyError *Error) {
	replyError = new(Error)

	b, _ := bson.Marshal(err)
	bson.Unmarshal(b, replyError)
	return
}
