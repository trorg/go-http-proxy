package proxy

import (
    "fmt"
    "net/http"
    "log"
    "io"
    "strings"
    "errors"
    "time"
)

var InternalServerError error = errors.New("internal server error")
var GatewayTimeoutError error = errors.New("gateway timeout")
var BadGatewayError error = errors.New("bad gateway")
var ServiceUnavailableError error = errors.New("service unavailable")

// ProxyHandler is function running before or after proxy request
type ProxyHandler func(next http.Handler) http.Handler

// Proxy
type Proxy struct {
    // upstream controls underlying servers and balancing strategy
    upstream    *Upstream

    // ErrorLog specifies an optional logger for errors accepting
    // connections, unexpected behavior from handlers, and
    // underlying FileSystem errors.
    // If nil, logging is done via the log package's standard logger.
    ErrorLog    *log.Logger

    // started stores started flag. prosy marked started on first request.
    started        bool

    // stop channel used to stop timers when proxy stops.
    stop           chan struct{}

    // beforeHandlers stores handlers running before proxyin
    beforeHandlers []ProxyHandler

    // afterHandlers stores handlers running after proxying
    afterHandlers  []ProxyHandler

    // tickers stores time tickers for errors checks
    tickers map[string]*time.Ticker
}

// NewProxy returns a new Proxy with configured upstream
func NewProxy(upstream *Upstream) *Proxy {
    return &Proxy{
        upstream: upstream,
        tickers: make(map[string]*time.Ticker),
    }
}

func (p *Proxy) Start() {
    p.started = true

    /**
    // TODO: where to place ticker
    ticker := time.Ticker(time.Second * errorsTimeout)
    server.ticker = ticker
    go func(){
        // TODO: stop timers on proxy stop
        select {
        case < t.C:
            server.mux.Lock()
            if server.errors >= server.maxErrors {
                server.online = false
            } else {
                server.online = true
            }
            server.errors = 0
            server.mux.Unlock()
        case <-server.stop:

        }
    }()
    */
}

func (p *Proxy) Stop() {
    p.started = false
}

// RegisterBeforeHandler adds ProxyHandler into handlers chain
// running before main request. Handlers run in FIFO order.
func (p *Proxy) RegisterBeforeHandler(h ProxyHandler) {
    p.beforeHandlers = append(p.beforeHandlers, h)
}

// BeforeHandlers returns slice of handlers running before proxying.
func (p *Proxy) BeforeHandlers() []ProxyHandler {
    return p.beforeHandlers
}

// RegisterAfterHandler adds ProxyHandler into handlers chain
// running after main request. Handlers run in FIFO order.
func (p *Proxy) RegisterAfterHandler(h ProxyHandler) {
    p.afterHandlers = append(p.afterHandlers, h)
}

// AfterHandlers returns slice of handlers running after proxying.
func (p *Proxy) AfterHandlers() []ProxyHandler {
    return p.afterHandlers
}

// GetHandler returns handler proxying http request.
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

// logf prints to the ErrorLog of the *Server associated with request r
// via ServerContextKey. If there's no associated server, or if ErrorLog
// is nil, logging is done via the log package's standard logger.
func (p *Proxy) logf(format string, args ...interface{}) {
    if p.ErrorLog != nil {
        p.ErrorLog.Printf(format, args...)
    } else {
        log.Printf(format, args...)
    }
}

// GetProxyHandler returns only proxy handler without middleware handlers.
func (p *Proxy) GetProxyHandler(next http.Handler) http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        complete := false
        for {
            server, err := p.upstream.next(r)
            if err != nil {
                p.logf("proxy: upstream [%s] : %v", server, err)
                http.Error(w, "Service Unavailable", 503)
                return
            }
            server.incrConnections()
            err = proxyRequest(server, w, r)
            server.decrConnections()
            if err != nil {
                server.incrErrors()
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

// finalHandler returns empty http.Handler used in handlers chain generation.
func finalHandler() http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
    })
}

// copyHeaders copying headers between http.Header instances.
func copyHeaders(src, dst http.Header) {
    for k, v := range src {
        dst.Set(k, strings.Join(v, ","))
    }
}

// proxyRequest proxying Request to specified Server.
// At this time it's don't intercept errors returned from backend.
func proxyRequest(server *UpstreamServer, w http.ResponseWriter, r *http.Request) error {
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

    if pres.StatusCode >= 500 {
        server.incrErrors()
    }

    copyHeaders(pres.Header, w.Header())
    w.WriteHeader(pres.StatusCode)
    _, err = io.Copy(w, pres.Body)
    if err != nil {
        return fmt.Errorf("%v: %w", err, BadGatewayError)
    }

    return nil
}
