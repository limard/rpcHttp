// Copyright 2009 The Go Authors. All rights reserved.
// Copyright 2012 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package jsonrpc2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
)

var ContentType = `application/json`

// ----------------------------------------------------------------------------
// Request and Response
// ----------------------------------------------------------------------------

// clientRequest represents a JSON-RPC request sent by a client.
type clientRequest struct {
	// JSON-RPC protocol.
	Version string `json:"jsonrpc"`

	// A String containing the name of the method to be invoked.
	Method string `json:"method"`

	// Object to pass as request parameter to the method.
	Params interface{} `json:"params"`

	// The request id. This can be of any type. It is used to match the
	// response with the request that it is replying to.
	Id uint64 `json:"id"`
}

// clientResponse represents a JSON-RPC response returned to a client.
type clientResponse struct {
	Id      interface{}      `json:"id"`
	Version string           `json:"jsonrpc"`
	Result  *json.RawMessage `json:"result"`
	Error   *json.RawMessage `json:"error"`
}

// encodeClientRequest encodes parameters for a JSON-RPC client request.
func encodeClientRequest(method string, args interface{}) ([]byte, error) {
	c := &clientRequest{
		Version: "2.0",
		Method:  method,
		Params:  args,
		Id:      uint64(rand.Int63()),
	}
	return json.Marshal(c)
}

// decodeClientResponse decodes the response body of a client request into
// the interface reply.
func decodeClientResponse(r io.Reader, reply interface{}) (e error) {
	var c clientResponse
	if err := json.NewDecoder(r).Decode(&c); err != nil {
		fmt.Println("decodeClientResponse.json.NewDecoder:", err)
		return &Error{
			Code:    E_PARSE,
			Message: err.Error()}
	}

	// Error
	if c.Error != nil {
		replyError := &Error{}
		if err := json.Unmarshal(*c.Error, replyError); err != nil {
			fmt.Println("decodeClientResponse.json.Unmarshal:", e)
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
			Message: ErrNullResult.Error(),
		}
	}
	if err := json.Unmarshal(*c.Result, reply); err != nil {
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
		fmt.Println("encodeClientRequest:", e)
		return &Error{
			Code:    E_INVALID_REQ,
			Message: err.Error()}
	}

	jsonReqBufR := bytes.NewReader(jsonReqBuf)
	rsp, err := http.Post(url, ContentType, jsonReqBufR)
	if err != nil {
		fmt.Println("http.Post:", e)
		return &Error{
			Code:    E_SERVER,
			Message: err.Error()}
	}

	defer rsp.Body.Close()

	return decodeClientResponse(rsp.Body, reply)
}

func CallEx(client *http.Client, url string, method string, request interface{}, reply interface{}) (e error) {
	jsonReqBuf, err := encodeClientRequest(method, request)
	if err != nil {
		fmt.Println("encodeClientRequest:", e)
		return &Error{
			Code:    E_INVALID_REQ,
			Message: err.Error()}
	}

	jsonReqBufR := bytes.NewReader(jsonReqBuf)
	rsp, err := client.Post(url, ContentType, jsonReqBufR)
	if err != nil {
		fmt.Println("http.Post:", e)
		return &Error{
			Code:    E_SERVER,
			Message: err.Error()}
	}

	defer rsp.Body.Close()

	return decodeClientResponse(rsp.Body, reply)
}

func ConvertError(err error) (replyError *Error) {
	replyError = new(Error)

	b, _ := json.Marshal(err)
	json.Unmarshal(b, replyError)
	return
}
