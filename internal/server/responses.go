package server

import (
	"encoding/json"
	"net/http"
)

type JSONResponse[T any] struct {
	Ok    bool   `json:"ok"`
	Data  T      `json:"data"`
	Error string `json:"error"`
	Meta  `json:"meta"`
}

type Meta struct {
	ReadSource     string `json:"read_source,omitempty"`
	Total          int    `json:"total,omitempty"`
	SQLiteFallback bool   `json:"sqlite_fallback,omitempty"`
}

func writeJSON[T any](w http.ResponseWriter, status int, data T, meta Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(JSONResponse[T]{
		Ok:   status >= 200 && status < 300,
		Data: data,
		Meta: meta,
	})
}

func writeError(w http.ResponseWriter, status int, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(JSONResponse[any]{
		Ok:    false,
		Data:  nil,
		Error: msg,
		Meta:  Meta{},
	})
}
