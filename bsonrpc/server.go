package bsonrpc

import (
	"bissoft/rpcHttp"
	"gopkg.in/mgo.v2/bson"
	"io/ioutil"
	"log"
	"net/http"
)

func NewCodec() *Codec {
	return &Codec{encSel: rpcHttp.DefaultEncoderSelector}
}

// Codec creates a CodecRequest to process each request.
type Codec struct {
	encSel rpcHttp.EncoderSelector
}

type serverRequest struct {
	Version string      `bson:"msgpackrpc"`
	Method  string      `bson:"method"`
	Params  interface{} `bson:"params"`
	Id      interface{} `bson:"id"`
}

type serverResponse struct {
	Version string      `bson:"msgpackrpc"`
	Result  interface{} `bson:"result,omitempty"`
	Error   interface{} `bson:"error,omitempty"`
	Id      interface{} `bson:"id"`
}

func (c *Codec) NewRequest(r *http.Request) rpcHttp.CodecRequest {
	var err error
	req := new(serverRequest)
	buf, e := ioutil.ReadAll(r.Body)
	if e != nil {
		err = &Error{
			Code:    E_PARSE,
			Message: e.Error(),
			Data:    req,
		}
	}
	e = bson.Unmarshal(buf, &req)
	if e != nil {
		err = &Error{
			Code:    E_PARSE,
			Message: e.Error(),
			Data:    req,
		}
	}
	r.Body.Close()
	return &CodecRequest{request: req, err: err, encoder: c.encSel.Select(r)}
}

type CodecRequest struct {
	request *serverRequest
	err     error
	encoder rpcHttp.Encoder
}

func (c *CodecRequest) Method() (string, error) {
	if c.err == nil {
		return c.request.Method, nil
	}
	return "", c.err
}

func (c *CodecRequest) ReadRequest(args interface{}) error {
	if c.err == nil && c.request.Params != nil {
		tempBuf, _ := bson.Marshal(c.request.Params)
		if err := bson.Unmarshal(tempBuf, args); err != nil {
			params := [1]interface{}{args}
			if err = bson.Unmarshal(tempBuf, &params); err != nil {
				log.Printf("ERROR: %s", string(tempBuf))
				c.err = &Error{
					Code:    E_INVALID_REQ,
					Message: err.Error(),
					Data:    c.request.Params,
				}
			}
		}
	}
	return c.err
}

// WriteResponse encodes the response and writes it to the ResponseWriter.
func (c *CodecRequest) WriteResponse(w http.ResponseWriter, reply interface{}) {
	res := &serverResponse{
		Version: "1.0",
		Result:  reply,
		Id:      c.request.Id,
	}
	c.writeServerResponse(w, res)
}

func (c *CodecRequest) WriteError(w http.ResponseWriter, status int, err error) {
	objErr, ok := err.(*Error)
	if !ok {
		objErr = &Error{
			Code:    E_SERVER,
			Message: err.Error(),
		}
	}
	res := &serverResponse{
		Version: "1.0",
		Error:   objErr,
		Id:      c.request.Id,
	}
	c.writeServerResponse(w, res)
}

func (c *CodecRequest) writeServerResponse(w http.ResponseWriter, res *serverResponse) {
	if c.request.Id != nil {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "application/bson; charset=utf-8")

		var buffer []byte
		var err error
		//buffer, err = bson.Marshal(c.request)

		buffer, err = bson.Marshal(res)
		w.Write(buffer)

		if err != nil {
			log.Println("bson Encode:", err.Error())
			rpcHttp.WriteError(w, 400, err.Error())
		}
	}
}
