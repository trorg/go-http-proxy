package proxy

import (
    "testing"
    "fmt"
    "net/http"
)

var servers []*UpstreamServer = []*UpstreamServer{
    NewUpstreamServer("http://127.0.0.1:8000", 1),
    NewUpstreamServer("http://127.0.0.1:8001", 1),
    NewUpstreamServer("http://127.0.0.1:8002", 1),
}

func TestStrategyRoundRobin_SetServers(t *testing.T) {
    strategy := StrategyRoundRobin{}
    strategy.SetServers(servers)

    r := strategy.ring
    for i := 0; i < r.Len(); i++ {
        srv := r.Value.(*UpstreamServer)
        if srv.String() != servers[i].String() {
            t.Errorf("Server addr is '%s'; want '%s'", srv.String(), servers[i].String())
        }

        p1 := srv
        p2 := servers[i]
        if fmt.Sprintf("%p", p1) != fmt.Sprintf("%p", p2) {
            t.Errorf("Server ptr is '%p'; want '%p'", p1, p2)
        }
        r = r.Next()
    }
}

func TestStrategyRoundRobin_Next(t *testing.T) {
    strategy := StrategyRoundRobin{}
    strategy.SetServers(servers)
    r, _ := http.NewRequest("GET", "http://127.0.0.1", nil)

    t.Run("Simple", func (t *testing.T) {
        for _, server := range servers {
            next, err := strategy.Next(r)
            if err != nil {
                t.Error(err)
            }

            if server.String() != next.String() {
                t.Errorf("server addr is '%s'; want '%s'", server.String(), next.String())
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
                next, err := strategy.Next(r)
                if err != nil {
                    t.Error(err)
                }
                if next.String() != servers[iter-1].String() {
                    t.Errorf("%d] Server is '%s'; want '%s'", i, next.String(), servers[iter-1].String())
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
        servers[0].online = false

        next, err := strategy.Next(r)
        if err != nil {
            t.Error(err)
        }
        if next.String() != servers[1].String() {
            t.Errorf("Server is '%s'; want '%s'", next.String(), servers[1].String())
        }
        servers[0].online = true
    })
}

func TestStrategyLeastConn_SetServers(t *testing.T) {
    strategy := StrategyLeastConn{}
    strategy.SetServers(servers)

    for i := 0; i < len(strategy.servers); i++ {
        srv := strategy.servers[i]
        if srv.String() != servers[i].String() {
            t.Errorf("Server addr is '%s'; want '%s'", srv.String(), servers[i].String())
        }

        p1 := srv
        p2 := servers[i]
        if fmt.Sprintf("%p", p1) != fmt.Sprintf("%p", p2) {
            t.Errorf("Server ptr is '%p'; want '%p'", p1, p2)
        }
    }
}

func TestStrategyLeastConn_Next(t *testing.T) {
    strategy := StrategyLeastConn{}
    strategy.SetServers(servers)

    r, _ := http.NewRequest("GET", "http://127.0.0.1", nil)
    t.Run("Simple", func (t *testing.T) {
        for _, server := range servers {
            next, err := strategy.Next(r)
            if err != nil {
                t.Error(err)
                return
            }

            if next.String() != server.String() {
                t.Errorf("server is '%s'; want '%s'", next.String(), server.String())
            }
            next.connections += 1
        }
    })
}

func TestStrategyConsistentHashing(t *testing.T) {
    strategy := StrategyConsistentHashing{
        GetKey: func (r *http.Request) (string, error) {
            return r.RequestURI, nil
        },
    }
    strategy.SetServers(servers)

    t.Run("SetServers", func (t *testing.T) {
        points := len(servers)*int(strategy.KetamaPoints)
        if len(strategy.points) != points {
            t.Errorf("generated points is '%d'; want '%d'", len(strategy.points), points)
        }
    })

    t.Run("getServers", func (t *testing.T) {
        wantServers := uint(2)
        servers := strategy.getServers("zzzztestkey", wantServers)

        if len(servers) != 2 {
            t.Errorf("servers length is %d; want %d", len(servers), wantServers)
        }

        if servers[0] == servers[1] {
            t.Errorf("Servers are not different.")
        }
    })

    t.Run("Next", func (t *testing.T) {
        r1, err  := http.NewRequest("GET", "http://127.0.0.1/a/b", nil)
        r2, err  := http.NewRequest("GET", "http://127.0.0.1/z/e", nil)
        if err != nil {
            t.Error(err)
        }

        reqs := []*http.Request{r1, r2}

        for _, r := range reqs {
            s1, err := strategy.Next(r)
            s2, err := strategy.Next(r)
            if err != nil {
                t.Errorf("no next server: %w", err)
            }

            if s1 != s2 {
                t.Errorf("Servers are different")
            }
        }
    })
}
