package proxy

import (
    "sync"
)

type ServerStatus struct {
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

func (s *ServerStatus) incrConnections() {
    s.mux.Lock()
    defer s.mux.Unlock()
    s.connections += 1
}

func (s *ServerStatus) decrConnections() {
    s.mux.Lock()
    defer s.mux.Unlock()
    s.connections -= 1
}

// Upstream server representation
type Server struct {
    addr   string

    weight uint8
    status *ServerStatus
}

func NewServer(addr string, weight uint8) Server {
    if weight == 0 {
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
    return s.status
}

// Upstream represents target servers where proxy can send requests
type Upstream struct {
    strategy UpstreamStrategy
    servers  []Server
}

// Create new Upstream
func NewUpstream(servers []Server, strategy UpstreamStrategy) Upstream {
    s := make([]Server, len(servers))
    copy(s, servers)
    strategy.SetServers(s)
    return Upstream{
        servers: s,
        strategy: strategy,
    }
}

func (u *Upstream) Strategy() UpstreamStrategy {
    return u.strategy
}

func (u *Upstream) Servers() []Server {
    ret := make([]Server, len(u.servers))
    copy(ret, u.servers)
    return ret
}

func (u *Upstream) next() (*Server, error) {
    return u.strategy.Next()
}
