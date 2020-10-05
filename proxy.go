package proxy

import (
    "fmt"
    "net/http"
    "log"
    "io"
    "strings"
)

// Server represents upstream server
type Server struct {
    Addr   string
    Weight int
}

func NewServer(addr string, weight int) Server {
    return Server{
        Addr: addr,
        Weight: weight,
    }
}

func (s *Server) String() string {
    return s.Addr
}

// A Upstream represents target servers where proxy can send requests
type Upstream struct {
    servers map[string]Server
}

func NewUpstream() Upstream {
    return Upstream{
        servers: make(map[string]Server),
    }
}

func (u *Upstream) AddServer(server Server) {
    u.servers[server.Addr] = server
}

func (u *Upstream) RemoveServer(server Server) {
    delete(u.servers, server.Addr)
}

func (u *Upstream) Servers() (servers []Server) {
    for _, s := range u.servers {
        servers = append(servers, s)
    }

    return servers
}

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
    next := p.finalHandler()
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
        served := false
        for _, server := range p.upstream.Servers() {
            url := fmt.Sprintf("%s/%s", server.Addr, r.RequestURI)
            preq, err := http.NewRequestWithContext(r.Context(), r.Method, url, r.Body)
            if err != nil {
                p.logf("proxy: upstream request error [%s] : %s - %s", server.String(), r.RequestURI, err)
                continue
            }
            copyHeaders(r.Header, preq.Header)
            pres, err := http.DefaultClient.Do(preq)
            if err != nil {
                p.logf("proxy: upstream read error [%s] : %s - %s", server.String(), r.RequestURI, err)
                continue
            }
            defer pres.Body.Close()
            copyHeaders(pres.Header, w.Header())
            w.WriteHeader(pres.StatusCode)
            _, err = io.Copy(w, pres.Body)
            if err != nil {
                p.logf("proxy: upstream read error [%s] : %s - %s", server.String(), r.RequestURI, err)
                continue
            }
            served := true
            break
        }

        if served {
            next.ServeHTTP(w, r)
        }
    })
}

// Get empty final handler, without next argument
func (p *Proxy) finalHandler() http.Handler {
    return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
    })
}

// Copy headers between responses
func copyHeaders(src, dst http.Header) {
    for k, v := range src {
        dst.Set(k, strings.Join(v, ","))
    }
}

