package server

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/YatinKare/map-data-fetcher/gateway/internal/worker"
)

func newJavaProxy(target *url.URL, supervisor worker.Supervisor) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = handleProxyError
	if supervisor == nil {
		return proxy
	}

	return &managedProxy{
		proxy:      proxy,
		supervisor: supervisor,
	}
}

type managedProxy struct {
	proxy      http.Handler
	supervisor worker.Supervisor
}

func (p *managedProxy) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	release, err := p.supervisor.Acquire(request.Context())
	if err != nil {
		handleProxyError(response, request, err)
		return
	}
	defer release()

	p.proxy.ServeHTTP(response, request)
}

func handleProxyError(response http.ResponseWriter, request *http.Request, err error) {
	log.Printf("Java worker proxy failed: %v", err)
	writeJSON(response, http.StatusBadGateway, map[string]string{
		"error": "java_worker_unavailable",
	})
}
