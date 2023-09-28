package controllers

import (
	"fmt"
	"net/http"
)

func writeFatalError(w http.ResponseWriter, msg string, originalErr error) {
	w.WriteHeader(500)
	_, err := w.Write([]byte(fmt.Sprintf("Returning error to user msg=%s err %+v", msg, originalErr)))
	if err != nil {
		// Ignore connection Hijacked errors coming from websocket connection upgrade
		if err == http.ErrHijacked {
			return
		}
		fmt.Println(fmt.Sprintf("Fatal error occurred writing error response to users err=%+v", err))
	}
}

func writeError(statusCode int, msg string, w http.ResponseWriter) {
	w.WriteHeader(statusCode)
	_, err := w.Write([]byte(msg))
	if err != nil {
		writeFatalError(w, "fatal error writing error rsp", err)
	}
}
