package proxy

import (
    "testing"
    "fmt"
)

var servers []Server = []Server{
    NewServer("http://127.0.0.1:8000", 1),
    NewServer("http://127.0.0.1:8001", 1),
    NewServer("http://127.0.0.1:8002", 1),
}

func TestStrategyRoundRobin_SetServers(t *testing.T) {
    strategy := StrategyRoundRobin{}
    strategy.SetServers(servers)

    r := strategy.ring
    for i := 0; i < r.Len(); i++ {
        srv := r.Value.(*Server)
        if srv.Addr() != servers[i].Addr() {
            t.Errorf("Server addr is '%s'; want '%s'", srv.Addr(), servers[i].Addr())
        }

        p1 := srv
        p2 := &servers[i]
        if fmt.Sprintf("%p", p1) != fmt.Sprintf("%p", p2) {
            t.Errorf("Server ptr is '%p'; want '%p'", p1, p2)
        }
        r = r.Next()
    }
}

func TestStrategyRoundRobin_Next(t *testing.T) {
    strategy := StrategyRoundRobin{}
    strategy.SetServers(servers)

    t.Run("Simple", func (t *testing.T) {
        for _, server := range servers {
            next, err := strategy.Next()
            if err != nil {
                t.Error(err)
            }

            if server.Addr() != next.Addr() {
                t.Errorf("server addr is '%s'; want '%s'", server.Addr(), next.Addr())
            }
        }
    })

    t.Run("Weighted", func (t *testing.T) {
        strategy.ring.Move(1) // rewind ring to first server
        servers[0].weight = 3
        servers[1].weight = 5
        servers[2].weight = 2

        run := func (iter, cnt int) {
            for i := 1; i <= cnt; i++ {
                next, err := strategy.Next()
                if err != nil {
                    t.Error(err)
                }
                if next.Addr() != servers[iter-1].Addr() {
                    t.Errorf("%d] Server is '%s'; want '%s'", i, next.Addr(), servers[iter-1].Addr())
                }
            }
        }

        run(1, 3)
        run(2, 5)
        run(3, 2)
    })

    t.Run("SkipOffline", func (t *testing.T) {
        strategy.ring.Move(1)
        servers[0].weight = 1
        servers[1].weight = 1
        servers[2].weight = 1
        servers[0].status.online = false

        next, err := strategy.Next()
        if err != nil {
            t.Error(err)
        }
        if next.Addr() != servers[1].Addr() {
            t.Errorf("Server is '%s'; want '%s'", next.Addr(), servers[1].Addr())
        }
        servers[0].status.online = true
    })
}

func TestStrategyLeastConn_SetServers(t *testing.T) {
    strategy := StrategyLeastConn{}
    strategy.SetServers(servers)

    for i := 0; i < len(strategy.servers); i++ {
        srv := &strategy.servers[i]
        if srv.Addr() != servers[i].Addr() {
            t.Errorf("Server addr is '%s'; want '%s'", srv.Addr(), servers[i].Addr())
        }

        p1 := srv
        p2 := &servers[i]
        if fmt.Sprintf("%p", p1) != fmt.Sprintf("%p", p2) {
            t.Errorf("Server ptr is '%p'; want '%p'", p1, p2)
        }
    }
}

func TestStrategyLeastConn_Next(t *testing.T) {
    strategy := StrategyLeastConn{}
    strategy.SetServers(servers)

    t.Run("Simple", func (t *testing.T) {
        for _, server := range servers {
            next, err := strategy.Next()
            if err != nil {
                t.Error(err)
                return
            }

            if next.Addr() != server.Addr() {
                t.Errorf("server is '%s'; want '%s'", next.Addr(), server.Addr())
            }
            next.status.connections += 1
        }
    })
}
