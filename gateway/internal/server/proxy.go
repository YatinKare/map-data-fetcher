package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func newJavaProxy(target *url.URL) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = handleProxyError
	return proxy
}

func handleProxyError(response http.ResponseWriter, request *http.Request, err error) {
	log.Printf("Java worker proxy failed: %v", err)
	writeJSON(response, http.StatusBadGateway, map[string]string{
		"error": "java_worker_unavailable",
	})
}
