package proxy

import (
    "sync"
    "container/ring"
    "errors"
)

type UpstreamStrategy uint8
const (
    UpstreamStrategyRoundRobin UpstreamStrategy = 0 + iota
    UpstreamStrategyLeastConn
    UpstreamStrategyConsistent
)

type ServerStatus struct {

    // Weight counter, each request server by server increase this counter.
    // When it reaches server's weight, it should be reset
    wc          uint8

    mux         sync.Mutex
    errors      uint
    online      bool
    connections uint
}

func (s *ServerStatus) Online() bool {
    s.mux.Lock()
    defer s.mux.Unlock()
    return s.online
}

func (s *ServerStatus) Connections() uint {
    s.mux.Lock()
    defer s.mux.Unlock()
    return s.connections
}

func (s *ServerStatus) Errors() uint {
    s.mux.Lock()
    defer s.mux.Unlock()
    return s.errors
}

// Upstream server representation
type Server struct {
    mux     sync.Mutex
    addr    string

    // Server weight, minimum is 1, maximum 254
    // if 0 or 255 is passed, weight will be set to 1
    weight  uint8
    status  *ServerStatus
}

func NewServer(addr string, weight uint8) Server {
    if weight == 0 || weight == 255 {
        weight = 1
    }
    return Server{
        addr: addr,
        weight: weight,
        status: &ServerStatus{
            online: true,
            connections: 0,
            errors: 0,
        },
    }
}

// Attribute is read only, no needs in mutex
func (s *Server) Addr() string {
    return s.addr
}

// Attribute is read only, no needs in mutex
func (s *Server) Weight() uint8 {
    return s.weight
}

// Attribute is read only, no needs in mutex
func (s *Server) String() string {
    return s.addr
}

// Key is used to generate server's id
func (s *Server) Key() string {
    return s.addr
}

// Status gets server status
func (s *Server) Status() *ServerStatus {
    s.mux.Lock()
    defer s.mux.Unlock()
    return s.status
}

// Upstream represents target servers where proxy can send requests
type Upstream struct {
    strategy UpstreamStrategy
    servers  []Server
    ring     *ring.Ring
    mux      sync.Mutex
}

// Create new Upstream
func NewUpstream() Upstream {
    return Upstream{
        servers: make([]Server, 0),
        strategy: UpstreamStrategyRoundRobin,
    }
}

// rebuildRing creates ring of servers
// method used only in Add/Remove server
func (u *Upstream) rebuildRing() {
    r := ring.New(len(u.servers))
    for i := 0; i < len(u.servers); i++ {
        r.Value = &u.servers[i]
        r = r.Next()
    }

    u.ring = r
}

// Strategy returns upstream strategy
func (u *Upstream) Strategy() UpstreamStrategy {
    u.mux.Lock()
    defer u.mux.Unlock()
    return u.strategy
}

// SetStrategy sets upstream strategy
func (u *Upstream) SetStrategy(s UpstreamStrategy) *Upstream {
    u.mux.Lock()
    defer u.mux.Unlock()
    u.strategy = s
    return u
}

func (u *Upstream) Servers() []Server {
    u.mux.Lock()
    defer u.mux.Unlock()
    return u.servers
}

// AddServer adds server in linked list
func (u *Upstream) AddServer(server Server) {
    u.mux.Lock()
    defer u.mux.Unlock()
    for _, s := range u.servers {
        if s.Key() == server.Key() {
            return
        }
    }
    u.servers = append(u.servers, server)
    u.rebuildRing()
}

// RemoveServer removes server from linked list
func (u *Upstream) RemoveServer(server Server) {
    u.mux.Lock()
    defer u.mux.Unlock()
    for i, s := range u.servers {
        if s.Key() == server.Key() {
            u.servers = append(u.servers[:i], u.servers[i+1:]...)
            u.rebuildRing()
            return
        }
    }
}

func (u *Upstream) next() (*Server, error) {
    u.mux.Lock()
    defer u.mux.Unlock()

    // RoundRobin
    next := u.ring
    for i := 0; i < u.ring.Len(); i++ {
        srv, ok := next.Value.(*Server)
        if ok  {
            status := srv.Status()
            status.mux.Lock()
            //srv.mux.Lock()
            if srv != nil && status.online {
                counter := status.wc + 1
                if counter == 0 || counter + 1 >= srv.weight {
                    u.ring = next.Next()
                    status.wc = 0
                } else {
                    status.wc += 1
                }
                status.mux.Unlock()
                //srv.mux.Unlock()
                return srv, nil
            }
            next = next.Next()
        }
    }

    return nil, errors.New("no valid servers")
}
