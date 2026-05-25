package common

import (
	"encoding/json"
	"net/http"
)

// JSONResponse 统一 JSON 响应结构
type JSONResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON 输出 JSON 响应
func JSON(w http.ResponseWriter, code int, data interface{}) {
	resp := JSONResponse{
		Code:    code,
		Message: "success",
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

// JSONSuccess 输出成功响应（业务 code=0，HTTP 200）
func JSONSuccess(w http.ResponseWriter, data interface{}) {
	resp := JSONResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// JSONCreated 输出创建成功响应（业务 code=0，HTTP 201）
func JSONCreated(w http.ResponseWriter, data interface{}) {
	resp := JSONResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// JSONError 输出错误响应
func JSONError(w http.ResponseWriter, code int, message string) {
	resp := JSONResponse{
		Code:    code,
		Message: message,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

// JSONAppError 输出 AppError 错误响应
func JSONAppError(w http.ResponseWriter, err *AppError) {
	JSONError(w, err.Code, err.Message)
}

// Redirect 重定向
func Redirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	http.Redirect(w, r, url, code)
}
