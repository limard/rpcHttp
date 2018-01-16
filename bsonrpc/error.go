package bsonrpc

import "encoding/json"

const (
	E_PARSE       = -32700
	E_INVALID_REQ = -32600
	E_NO_METHOD   = -32601
	E_BAD_PARAMS  = -32602
	E_INTERNAL    = -32603
	E_SERVER      = -32000
)

type Error struct {
	Code    int         `bson:"code"`    /* required */
	Message string      `bson:"message"` /* required */
	Data    interface{} `bson:"data"`    /* optional */
}

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}