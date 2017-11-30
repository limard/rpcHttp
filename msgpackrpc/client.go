package msgpackrpc

import (
	"bytes"
	"io"
	"math/rand"
	"net/http"

	"github.com/vmihailenco/msgpack"
)

var ContentType = `application/msgpack`

// clientRequest represents a JSON-RPC request sent by a client.
type clientRequest struct {
	Version string      `msgpack:"msgpackrpc"`
	Method  string      `msgpack:"method"`
	Params  interface{} `msgpack:"params"`
	Id      interface{} `msgpack:"id"`
}

// clientResponse represents a JSON-RPC response returned to a client.
type clientResponse struct {
	Id      interface{} `msgpack:"id"`
	Version string      `msgpack:"msgpackrpc"`
	Result  interface{} `msgpack:"result"`
	Error   interface{} `msgpack:"error"`
}

func encodeClientRequest(method string, args interface{}) ([]byte, error) {
	c := &clientRequest{
		Version: "1.0",
		Method:  method,
		Params:  args,
		Id:      uint64(rand.Int63()),
	}
	return msgpack.Marshal(c)
}

func decodeClientResponse(r io.Reader, reply interface{}) (e error) {
	var c clientResponse
	if err := msgpack.NewDecoder(r).Decode(&c); err != nil {
		return &Error{
			Code:    E_PARSE,
			Message: err.Error()}
	}

	// Error
	if c.Error != nil {
		replyError := &Error{}
		tempBuf, _ := msgpack.Marshal(c.Error)
		if err := msgpack.Unmarshal(tempBuf, replyError); err != nil {
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
	tempBuf, _ := msgpack.Marshal(c.Result)
	if err := msgpack.Unmarshal(tempBuf, reply); err != nil {
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

	b, _ := msgpack.Marshal(err)
	msgpack.Unmarshal(b, replyError)
	return
}
