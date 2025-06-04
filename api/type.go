package api

import (
	"encoding/json"
	"net/http"
)

type CommonResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

func EncodeResponse(w http.ResponseWriter, response interface{}) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(response)
}

// Code define
const (
	OK                int32 = 200
	UnknownError      int32 = 2000
	RequestParamError int32 = 2001
	ServerError       int32 = 2002
)

// CodeMap is a mapping for code and error info
var CodeMap = map[int32]string{
	OK:                "Success",
	UnknownError:      "Unknown error",
	ServerError:       "Server error",
	RequestParamError: "Request params error",
}

type Error struct {
	Code    int32
	Message string
}

func NewError(code int32) *Error {
	return &Error{
		Code:    code,
		Message: CodeMap[code],
	}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}

	return e.Message
}

func GetError(err error) *Error {
	if e, ok := err.(*Error); ok {
		return e
	} else {
		e := &Error{}
		e.Code = UnknownError
		e.Message = err.Error()
		return e
	}
}
