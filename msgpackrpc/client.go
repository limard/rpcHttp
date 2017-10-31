package msgpackrpc

import (
	"bytes"
	"github.com/vmihailenco/msgpack"
	"io"
	"math/rand"
	"net/http"
)

// clientRequest represents a JSON-RPC request sent by a client.
type clientRequest struct {
	Version string      `msgpack:"msgpackrpc"`
	Method  string      `msgpack:"method"`
	Params  interface{} `msgpack:"params"`
	Id      uint64      `msgpack:"id"`
}

// clientResponse represents a JSON-RPC response returned to a client.
type clientResponse struct {
	Id      interface{} `msgpack:"id"`
	Version string      `msgpack:"jsonrpc"`
	Result  *[]byte     `msgpack:"result"`
	Error   *[]byte     `msgpack:"error"`
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
		if err := msgpack.Unmarshal(*c.Error, replyError); err != nil {
			return &Error{
				Code:    E_PARSE,
				Message: string(*c.Error),
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
	if err := msgpack.Unmarshal(*c.Result, reply); err != nil {
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
	rsp, err := http.Post(url, `application/msgpack`, jsonReqBufR)
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
