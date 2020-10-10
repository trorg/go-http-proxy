package proxy

import (
    "testing"
)

func TestServerStatus(t *testing.T) {
    status := ServerStatus{
        errors: 0,
        online: true,
        connections: 0,
    }

    t.Run("Online", func (t *testing.T) {
        online := status.Online()
        if !online {
            t.Errorf("online is %t; want true", online)
        }
    })

    t.Run("Connections", func (t *testing.T) {
        status := ServerStatus{
            connections: 10,
        }
        c := status.Connections()
        if c != 10 {
            t.Errorf("connections is %d; want 10", c)
        }
    })

    t.Run("Errors", func (t *testing.T) {
        status := ServerStatus{
            errors: 10,
        }
        c := status.Errors()
        if c != 10 {
            t.Errorf("errors is %d; want 10", c)
        }
    })
}

func TestServer(t *testing.T) {
    addr := "http://127.0.0.1:8080"
    weight := uint8(1)
    server := NewServer(addr, weight)

    t.Run("Addr", func (t *testing.T) {
        if server.Addr() != addr {
            t.Errorf("addr is '%s'; want '%s'", server.Addr(), addr)
        }
    })

    t.Run("Weight", func (t *testing.T) {
        if server.Weight() != weight {
            t.Errorf("addr is '%d'; want '%d'", server.Weight(), weight)
        }
    })

    t.Run("String", func (t *testing.T) {
        if server.String() != addr {
            t.Errorf("addr is '%s'; want '%s'", server.String(), addr)
        }
    })

    t.Run("Key", func (t *testing.T) {
        if server.Key() != addr {
            t.Errorf("key is '%s'; want '%s'", server.Key(), addr)
        }
    })

    t.Run("Status", func (t *testing.T) {
        status := server.Status()
        if !status.Online() {
            t.Errorf("online is '%t'; want 'tru'", status.Online())
        }
    })
}

func TestUpstream(t *testing.T) {
    server1 := NewServer("http://127.0.0.1:8080", 1)
    server2 := NewServer("http://127.0.0.1:8081", 1)
    strategy := StrategyRoundRobin{}
    upstream := NewUpstream([]Server{server1, server2}, &strategy)

    t.Run("Servers", func (t *testing.T) {
        servers := upstream.Servers()
        if len(servers) != 2 {
            t.Errorf("servers length is %d; want %d", len(servers), 2)
        }
    })

    // next is main functio which select proper server
    // there are two servers in upstream, they was added in prev test
    t.Run("next", func (t *testing.T) {
        t.Run("RoundRobin", func(t *testing.T) {
            for _, srv := range upstream.Servers() {
                next, err := upstream.next()
                if err != nil {
                    t.Error(err)
                }
                if next == nil {
                    t.Fatalf("Next is nil")
                }
                if srv.Key() != next.Key() {
                    t.Errorf("RoundRobin server is '%s'; want '%s'", srv.Key(), next.Key())
                }
            }
        })
    })
}
