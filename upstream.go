package proxy

import (
    "fmt"
    "sync"
    "strings"
    "strconv"
    "net"
    "net/http"
    "time"
)

// A Server is representation of upstream server.
type UpstreamServer struct {
    // host is network name or ip address of server.
    host string

    // port
    port uint16

    // proto is server's protocol - http or https
    proto  string

    // weight
    weight uint8

    // maxErrors default 1
    maxErrors     uint

    // errorsTimeout default 10 seconds
    errorsTimeout uint

    // online status
    online bool

    // errors is errors counter
    errors uint

    // connections is active connections count
    connections uint

    // done channel controls ticker stopping
    stop chan struct{}

    // ticker is time.Ticker, used to verify errors rate in time
    ticker *time.Ticker

    mux    sync.Mutex
}

// NewUpstreamServer returns Server with assigned address and weight.
// Address format is http(s)://host:port
func NewUpstreamServer(addr string, weight uint8) *UpstreamServer {
    if weight == 0 {
        weight = 1
    }

    proto := "http"
    if strings.HasPrefix(addr, "https") {
        proto = "https"
    }

    s := strings.TrimPrefix(addr, fmt.Sprintf("%s://", proto))
    host, sport, err := net.SplitHostPort(s)
    if err != nil {
        panic(fmt.Errorf("can't create server: %w", err))
    }

    port, err := strconv.Atoi(sport)
    if err != nil {
        panic(fmt.Errorf("can't parse port: %w", err))
    }

    return &UpstreamServer{
        proto: proto,
        host: host,
        port: uint16(port),
        weight: weight,
        maxErrors: 1,
        errorsTimeout: 10,
        online: true,
        errors: 0,
        connections: 0,
    }
}

// Host returns server's host.
func (u UpstreamServer) Host() string {
    return u.host
}

// Port returns server's port.
func (u UpstreamServer) Port() uint16 {
    return u.port
}

// Proto returns server's proto.
func (u UpstreamServer) Proto() string {
    return u.proto
}

// SetWeight sets server's weight.
func (u *UpstreamServer) SetWeight(w uint8) *UpstreamServer {
    u.mux.Lock()
    defer u.mux.Unlock()
    if w == 0 {
        w = 1
    }
    u.weight = w
    return u
}

// Weight returns server's weight.
func (u UpstreamServer) Weight() uint8 {
    return u.weight
}

// SetMaxErrors sets maximum errors in ErrorsTimeout interval then the server
// goes offline for ErrorsTimeout seconds.
func (u *UpstreamServer) SetMaxErrors(e uint) *UpstreamServer {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.maxErrors = e
    /*
    if e == 0 {
        u.ticker.Stop()
    } else {
        u.ticker.Reset(time.Second * u.errorsTimeout)
    }
    */
    return u
}

// GetMaxErrors returns maximum errors.
func (u UpstreamServer) MaxErrors() uint {
    return u.maxErrors
}

// SetErrorsTimeout sets time in secods. If during this time server's errors 
// reach maxErrors, server goes offline for this period.
func (u *UpstreamServer) SetErrorsTimeout(t uint) *UpstreamServer {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.errorsTimeout = t
    /*
    if t == 0 {
        u.ticker.Stop()
    } else {
        u.ticker.Reset(time.Second * t)
    }
    */
    return u
}

// GetErrorsTimeout returns time in seconds.
func (u UpstreamServer) ErrorsTimeout() uint {
    return u.errorsTimeout
}

// incrConnections increments server's connections.
func (u *UpstreamServer) incrConnections() {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.connections += 1
}

// decrConnections decrement server's connections.
func (u *UpstreamServer) decrConnections() {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.connections -= 1
}

func (u *UpstreamServer) Online() bool {
    return u.online
}

// Errors returns server errors number.
func (u *UpstreamServer) Errors() uint {
    u.mux.Lock()
    defer u.mux.Unlock()
    return u.errors
}

// incrErrors increments server's connections.
func (u *UpstreamServer) incrErrors() {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.errors += 1
}

// decrErrors decrement server's connections.
func (u *UpstreamServer) decrErrors() {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.errors -= 1
}

// String returns string representations os server.
func (u *UpstreamServer) String() string {
    return fmt.Sprintf("%s://%s:%d", u.proto, u.host, u.port)
}

// A Upstream defines parameters for Proxy.
type Upstream struct {
    servers  []*UpstreamServer
    strategy UpstreamStrategy
}

// Create new Upstream
func NewUpstream(servers []*UpstreamServer, strategy UpstreamStrategy) *Upstream {
    strategy.SetServers(servers)
    return &Upstream{
        servers: servers,
        strategy: strategy,
    }
}

// StartTimer create ticker for each server with ErrorsTimeout and MaxErrors 
// greater than zero.
func (u *Upstream) StartTimers() {
    for i := range u.servers {
        s := u.servers[i]
        if s.ErrorsTimeout() > 0 && s.MaxErrors() > 0 {
            //t := time.NewTicker(time.Second * s.errorsTimeout)
        }
    }
}

// Strategy returns upstream strategy.
func (u *Upstream) Strategy() UpstreamStrategy {
    return u.strategy
}

// Servers returns upstream servers.
func (u *Upstream) Servers() []*UpstreamServer {
    ret := make([]*UpstreamServer, 0)
    copy(ret, u.servers)
    return ret
}

// next returns server for request processing.
func (u *Upstream) next(r *http.Request) (*UpstreamServer, error) {
    return u.strategy.Next(r)
}
