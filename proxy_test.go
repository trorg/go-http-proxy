package proxy

import (
    "testing"
    "net/http"
    "fmt"
    "io/ioutil"
    "strings"
)

var DefaultServers []Server = []Server{
    NewServer("http://127.0.0.1:8080", 1),
    NewServer("http://127.0.0.1:8081", 1),
    NewServer("http://127.0.0.1:8082", 1),
}

func getDefaultUpstream() Upstream {
    strategy := StrategyRoundRobin{}
    u := NewUpstream(DefaultServers, &strategy)
    return u
}

func startBackends(done chan struct{}) {
    for _, server := range DefaultServers {
        handler := func (srv Server) http.Handler {
            return http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
                w.Header().Set("X-Proxied-Header", "1")
                w.Header().Set("X-Server", srv.Addr())
                fmt.Fprintf(w, "hello")
            })
        }(server)

        srv := &http.Server{
            Addr: strings.Replace(server.Addr(), "http://", "", 1),
            Handler: handler,
        }
        go srv.ListenAndServe()
        go func(srv *http.Server){
            <-done
            srv.Close()
        }(srv)
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
        t.Fatalf("registered handlers is %d; want %d", len(handlers), 3)
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
        t.Fatalf("registered handlers is %d; want %d", len(handlers), 3)
    }
}

func TestProxy_GetHandler(t *testing.T) {
    done := make(chan struct{})
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

    server := &http.Server{
        Addr: "127.0.0.1:9000",
        Handler: proxy.GetHandler(),
    }
    go server.ListenAndServe()
    defer server.Close()
    startBackends(done)

    for _, server := range DefaultServers {
        resp, err := http.Get("http://127.0.0.1:9000/")
        if err != nil {
            t.Error(err)
        }
        defer resp.Body.Close()
        body, err := ioutil.ReadAll(resp.Body)
        if err != nil {
            t.Error(err)
        }

        header := resp.Header.Get("X-Server")

        if header != server.Addr() {
            t.Fatalf("X-Server is '%s'; want '%s'", header, server.Addr())
        }

        if resp.Header.Get("X-Proxied-Header") == "" {
            t.Fatalf("X-Proxied-Header is empty; want %s", "1")
        }

        if string(body) != "hello" {
            t.Fatal("Body not proxied")
        }

        if len(order) != 3 {
            t.Fatal("Before handlers are not runned")
        }

        if order[0] != "first" || order[1] != "second" || order[2] != "last" {
            t.Fatal("Before handlers order mismatch")
        }
        order = []string{}
    }
    done <- struct{}{}
}
