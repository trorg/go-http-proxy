package proxy

import (
    "container/ring"
    "sync"
    "errors"
)

type UpstreamStrategy interface {
    SetServers(servers []Server)
    Next() (*Server, error)
}

type StrategyRoundRobin struct {
    // Weight counter, each request server by server increase this counter.
    // When it reaches server's weight, it should be reset
    wc   uint16

    ring *ring.Ring
    mux  sync.Mutex
}

func (s *StrategyRoundRobin) SetServers(servers []Server) {
    s.mux.Lock()
    defer s.mux.Unlock()

    r := ring.New(len(servers))
    for i := 0; i < len(servers); i++ {
        r.Value = &servers[i]
        r = r.Next()
    }

    s.ring = r
}

func (s *StrategyRoundRobin) Next() (*Server, error) {
    s.mux.Lock()
    defer s.mux.Unlock()

    // RoundRobin
    next := s.ring
    for i := 0; i < s.ring.Len(); i++ {
        srv, ok := next.Value.(*Server)
        if ok  {
            // lock srv?
            if srv != nil && srv.status.online {
                s.wc += 1
                if s.wc == 0 || s.wc >= uint16(srv.weight) {
                    s.ring = next.Next()
                    s.wc = uint16(0)
                }
                return srv, nil
            }
            next = next.Next()
        }
    }

    return nil, errors.New("no valid server")
}

//
// LeastConnStrategy
//

type StrategyLeastConn struct {
    mux     sync.Mutex
    servers []Server
}

func (s *StrategyLeastConn) SetServers(servers []Server) {
    s.mux.Lock()
    defer s.mux.Unlock()
    s.servers = servers
}

func (s *StrategyLeastConn) Next() (*Server, error) {
    if len(s.servers) < 1 {
        return nil, errors.New("empty upstreams")
    }

    var next *Server
    var err error
    min := s.servers[0].status.connections
    for i := range s.servers {
        srv := &s.servers[i]
        if !srv.status.online {
            continue
        }

        if srv.status.connections == 0 {
            return srv, nil
        }

        x := srv.status.connections - uint(srv.weight)
        if x < min {
            min = x
            next = srv
        }
    }

    if next == nil {
        err = errors.New("no valid server")
    }
    return next, err
}
