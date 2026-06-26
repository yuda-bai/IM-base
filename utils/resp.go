package utils

import (
	"net/http"

	"github.com/goccy/go-json"
)

//分页

type H struct {
	Code  int         `json:"code"`
	Msg   string      `json:"msg"`
	Data  interface{} `json:"data"`
	Rows  interface{} `json:"rows"`
	Total interface{}
}

// Resp 成功
func Resp(w http.ResponseWriter, code int, data interface{}, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	h := H{
		Code: code,
		Data: data,
		Msg:  msg,
	}
	ret, err := json.Marshal(h)
	if err != nil {
		return
	}
	_, err = w.Write(ret)
	if err != nil {
		return
	}
}

// RespList 分页列表响应
func RespList(w http.ResponseWriter, code int, data interface{}, total interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	h := H{
		Code:  code,
		Rows:  data,
		Total: total,
	}
	ret, err := json.Marshal(h)
	if err != nil {
		return
	}
	_, err = w.Write(ret)
	if err != nil {
		return
	}
}

// RespOk 列表分页成功
func RespOk(w http.ResponseWriter, data interface{}, msg string) {
	Resp(w, 0, data, msg)
}

// RespFail 列表分页失败
func RespFail(w http.ResponseWriter, msg string) {
	Resp(w, -1, nil, msg)
}

// RespOkList 分页成功
func RespOkList(w http.ResponseWriter, data interface{}, total interface{}) {
	RespList(w, 0, data, total)
}
