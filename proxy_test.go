package proxy

import (
    "testing"
    "net/http"
    "fmt"
    "io/ioutil"
)

var DefaultServerAddr string = "http://127.0.0.1:9001"
var DefaultServerWeight int = 1

func getDefaultUpstream() Upstream {
    s := NewServer(DefaultServerAddr, DefaultServerWeight)
    u := NewUpstream()
    u.AddServer(s)
    return u
}

func getDefaultBackendServer() *http.Server {
    return &http.Server{
        Addr: "127.0.0.1:9001",
        Handler: http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            w.Header().Set("X-Proxied-Header", "1")
            fmt.Fprintf(w, "hello")
        }),
    }
}

func TestServer_New(t *testing.T) {
    server := NewServer(DefaultServerAddr, DefaultServerWeight)
    if server.Addr != DefaultServerAddr {
        t.Errorf("Addr is '%s'; want '%s'", server.Addr, DefaultServerAddr)
    }

    if server.Weight != DefaultServerWeight {
        t.Errorf("Weigh is %d; want %d", server.Weight, DefaultServerWeight)
    }
}

func TestNewUpstream(t *testing.T) {
    server := NewServer(DefaultServerAddr, DefaultServerWeight)
    upstream := NewUpstream()
    upstream.AddServer(server)
}

func TestUpstream_AddServer(t *testing.T) {
    server := NewServer("1", 1)
    upstream := NewUpstream()
    upstream.AddServer(server)
    upstream.AddServer(server)
    upstream.AddServer(server)
    servers := upstream.Servers()

    if len(servers) != 1 {
        t.Errorf("servers length is %d; want %d", len(servers), 1)
    }
}

func TestUpstream_RemoveServer(t *testing.T) {
    server1 := NewServer("1", 1)
    server2 := NewServer("2", 1)
    upstream := NewUpstream()
    upstream.AddServer(server1)
    upstream.AddServer(server2)
    upstream.RemoveServer(server1)
    upstream.RemoveServer(server1)
    upstream.RemoveServer(server1)
    servers := upstream.Servers()

    if len(servers) != 1 {
        t.Errorf("servers length is %d; want %d", len(servers), 1)
    }
}

func TestUpstream_Servers(t *testing.T) {
    server := NewServer("1", 1)
    server2 := NewServer("2", 1)
    upstream := NewUpstream()
    upstream.AddServer(server)
    upstream.AddServer(server)
    upstream.AddServer(server2)
    servers := upstream.Servers()

    if len(servers) != 2 {
        t.Errorf("servers length is %d; want %d", len(servers), 2)
    }
}

func TestNewProxy(t *testing.T) {
    u := getDefaultUpstream()
    proxy := NewProxy(u)

    if proxy == nil {
        t.Error("Nil proxy")
    }
}

func TestProxy_AddBeforeHandler(t *testing.T) {
    u := getDefaultUpstream()
    proxy := NewProxy(u)

    handler := func (next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        })
    }

    proxy.RegisterBeforeHandler(handler)
    proxy.RegisterBeforeHandler(handler)
    proxy.RegisterBeforeHandler(handler)

    handlers := proxy.BeforeHandlers()
    if len(handlers) != 3 {
        t.Errorf("registered handlers is %d; want %d", len(handlers), 3)
    }
}

func TestProxy_AddAfterHandler(t *testing.T) {
    u := getDefaultUpstream()
    proxy := NewProxy(u)

    handler := func (next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
        })
    }

    proxy.RegisterAfterHandler(handler)
    proxy.RegisterAfterHandler(handler)
    proxy.RegisterAfterHandler(handler)

    handlers := proxy.AfterHandlers()
    if len(handlers) != 3 {
        t.Errorf("registered handlers is %d; want %d", len(handlers), 3)
    }
}

func TestProxy_GetHandler(t *testing.T) {
    order := []string{}
    u := getDefaultUpstream()
    proxy := NewProxy(u)
    proxy.RegisterBeforeHandler(func (next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            order = append(order, "first")
            next.ServeHTTP(w, r)
        })
    })
    proxy.RegisterBeforeHandler(func (next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            order = append(order, "second")
            next.ServeHTTP(w, r)
        })
    })
    proxy.RegisterAfterHandler(func (next http.Handler) http.Handler {
        return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
            order = append(order, "last")
        })
    })

    backendServer := getDefaultBackendServer()
    server := &http.Server{
        Addr: "127.0.0.1:9000",
        Handler: proxy.GetHandler(),
    }
    go backendServer.ListenAndServe()
    go server.ListenAndServe()
    defer server.Close()
    defer backendServer.Close()

    resp, err := http.Get("http://127.0.0.1:9000/")
    if err != nil {
        t.Error(err)
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        t.Error(err)
    }

    if resp.Header.Get("X-Proxied-Header") == "" {
        t.Errorf("X-Proxied-Header is empty; want %s", "1")
    }

    if string(body) != "hello" {
        t.Error("Body not proxied")
    }

    if len(order) != 3 {
        t.Error("Before handlers are not runned")
    }

    if order[0] != "first" || order[1] != "second" || order[2] != "last" {
        t.Error("Before handlers order mismatch")
    }
}
