package proxy

import (
    "fmt"
    "net/http"
    "log"
    "io"
    "strings"
    "errors"
)

var InternalServerError error = errors.New("internal server error")
var GatewayTimeoutError error = errors.New("gateway timeout")
var BadGatewayError error = errors.New("bad gateway")
var ServiceUnavailableError error = errors.New("service unavailable")

// Middleware handler, used to insert user defined actions 
// before and after main request
type ProxyHandler func(next http.Handler) http.Handler

// Proxy
type Proxy struct {
    upstream    Upstream
    ErrorLog    *log.Logger

    beforeHandlers []ProxyHandler
    afterHandlers  []ProxyHandler
}

// Creates new proxy instance
func NewProxy(upstream Upstream) *Proxy {
    return &Proxy{
        upstream: upstream,
    }
}

func (p *Proxy) RegisterBeforeHandler(h ProxyHandler) {
    p.beforeHandlers = append(p.beforeHandlers, h)
}

func (p *Proxy) BeforeHandlers() []ProxyHandler {
    return p.beforeHandlers
}

func (p *Proxy) RegisterAfterHandler(h ProxyHandler) {
    p.afterHandlers = append(p.afterHandlers, h)
}

func (p *Proxy) AfterHandlers() []ProxyHandler {
    return p.afterHandlers
}

// Generate http.Handler from middlewares and main 
// proxy handler
func (p *Proxy) GetHandler() http.Handler {
    next := finalHandler()
    for i := len(p.afterHandlers) - 1; i >= 0; i-- {
        next = p.afterHandlers[i](next) 
    }
    next = p.GetProxyHandler(next)
    for i := len(p.beforeHandlers) - 1; i >= 0; i-- {
        next = p.beforeHandlers[i](next)
    }

    return next
}

func (p *Proxy) logf(format string, args ...interface{}) {
    if p.ErrorLog != nil {
        p.ErrorLog.Printf(format, args...)
    } else {
        log.Printf(format, args...)
    }
}

// Main proxy handler
func (p *Proxy) GetProxyHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        // @TODO:
        // - Mark server online/offline
        // - Select next server by Upstream.Next()
        complete := false
        for {
            server, err := p.upstream.next()
            if err != nil {
                p.logf("proxy: upstream [%s] : %v", server, err)
                http.Error(w, "Service Unavailable", 503)
                return
            }
            server.status.incrConnections()
            err = proxyRequest(server, w, r)
            server.status.decrConnections()
            if err != nil {
                p.logf("proxy: upstream [%s] : %v", server, err)
            } else {
                complete = true
                break
            }
        }

        if complete {
            next.ServeHTTP(w, r)
        }
    })
}

// Final empty handler
func finalHandler() http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
    })
}

// Copy headers between http.Header instances
func copyHeaders(src, dst http.Header) {
    for k, v := range src {
        dst.Set(k, strings.Join(v, ","))
    }
}

// Proxy request to serveer
func proxyRequest(server *Server, w http.ResponseWriter, r *http.Request) error {
    url := fmt.Sprintf("%s/%s", server.String(), r.RequestURI)
    preq, err := http.NewRequestWithContext(r.Context(), r.Method, url, r.Body)
    if err != nil {
        return fmt.Errorf("%v: %w", err, InternalServerError)
    }
    copyHeaders(r.Header, preq.Header)
    pres, err := http.DefaultClient.Do(preq)
    if err != nil {
        return fmt.Errorf("%v: %w", err, GatewayTimeoutError)
    }
    defer pres.Body.Close()
    copyHeaders(pres.Header, w.Header())
    w.WriteHeader(pres.StatusCode)
    _, err = io.Copy(w, pres.Body)
    if err != nil {
        return fmt.Errorf("%v: %w", err, BadGatewayError)
    }

    return nil
}
