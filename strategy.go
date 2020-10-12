package proxy

import (
    "container/ring"
    "sync"
    "errors"
    "hash/crc32"
    "sort"
    "net/http"
    "fmt"
)

// UpstreamStrategy describes interface used to balancing requests to
// underlying servers.
type UpstreamStrategy interface {
    // SetServers  
    SetServers(servers []*UpstreamServer)

    // Next should returns Server for processing request.
    Next(r *http.Request) (*UpstreamServer, error)
}

// A StrategyRoundRobin realises UpstreamStrategy. Requests are served by server
// in sequence.
type StrategyRoundRobin struct {
    // wc is weight counter, each request served by server increase this counter.
    // When it reaches server's weight, it should be reset
    wc   uint

    // ring stores servers
    ring *ring.Ring
    mux  sync.Mutex
}

// SetServers creates new Ring filled with servers.
// Method is safe for concurrent access.
func (s *StrategyRoundRobin) SetServers(servers []*UpstreamServer) {
    s.mux.Lock()
    defer s.mux.Unlock()

    r := ring.New(len(servers))
    for i := 0; i < len(servers); i++ {
        r.Value = servers[i]
        r = r.Next()
    }

    s.ring = r
}

// Next returns online server in sequence.
// Method is safe for concurrent access.
func (us *StrategyRoundRobin) Next(r *http.Request) (*UpstreamServer, error) {
    us.mux.Lock()
    defer us.mux.Unlock()

    next := us.ring
    for i := 0; i < us.ring.Len(); i++ {
        srv, ok := next.Value.(*UpstreamServer)
        if ok  {
            if srv != nil && srv.online {
                us.wc += 1
                if us.wc == 0 || us.wc >= uint(srv.weight) {
                    us.ring = next.Next()
                    us.wc = 0
                }
                return srv, nil
            }
            next = next.Next()
        }
    }

    return nil, errors.New("no valid servers")
}

// A StrategyLeastConn realises UpstreamStrategy. Requests are served by server
// with least connections count.
type StrategyLeastConn struct {
    mux     sync.Mutex
    servers []*UpstreamServer
}

// SetServers sets servers to manage in Next method.
// Method is safe for concurrent access.
func (s *StrategyLeastConn) SetServers(servers []*UpstreamServer) {
    s.mux.Lock()
    defer s.mux.Unlock()
    s.servers = servers
}

// Next resturns first online server with least number of active connections.
// Method is safe for concurrent access.
func (s *StrategyLeastConn) Next(r *http.Request) (*UpstreamServer, error) {
    if len(s.servers) < 1 {
        return nil, errors.New("empty upstreams")
    }

    var next *UpstreamServer
    var err error
    min := s.servers[0].connections
    for i := range s.servers {
        srv := s.servers[i]
        if !srv.online {
            continue
        }

        if srv.connections == 0 {
            return srv, nil
        }

        if srv.connections < min {
            min = srv.connections
            next = srv
        }
    }

    if next == nil {
        err = errors.New("no valid servers")
    }
    return next, err
}

// A StrategyConsistentHashing realises UpstreamStrategy. Server is selected in
// static way by ketama hashing algorithm. The strategy  ensures that only a
// few keys will be remapped to different servers when a server is added to or
// removed from the upstream. This strategy is compatible with Nginx consistent
// hashing.
type StrategyConsistentHashing struct {
    mux    sync.Mutex
    points []KetamaPoint

    // KetamaPoints defines how many points are generated for each server.
    // Default value is 180.
    KetamaPoints uint

    // BackupCount is count of servers returned for hashing key generated in
    // GetKey method. Default is 2
    BackupCount uint

    // GetKey returns hashing key from http.Request structure.
    // It must be realised by user.
    GetKey func (r *http.Request) (string, error)
}

type KetamaPoint struct {
    hash   uint32
    server *UpstreamServer
}

// SetServers set strategy servers.
func (s *StrategyConsistentHashing) SetServers(servers []*UpstreamServer) {
    s.mux.Lock()
    defer s.mux.Unlock()

    if s.BackupCount == 0 {
        s.BackupCount = 2
    }

    if s.KetamaPoints == 0 {
        s.KetamaPoints = 180
    }

    // generate points
    for i := range servers {
        var phash, hash uint32

        srv := servers[i]
        n := uint(srv.weight) * s.KetamaPoints

        for i := uint(0); i < n; i++ {
            hash = crc32.ChecksumIEEE([]byte(fmt.Sprintf("%s\\0%d%d", srv.Host(), srv.Port(), phash)))
            s.points = append(s.points, KetamaPoint{hash, srv})
            phash = hash
        }
    }

    // sort points
    sort.Slice(s.points, func (i, j int) bool {
        return s.points[i].hash < s.points[j].hash
    })

    for i, j := 0, 1; j < len(s.points); j++ {
        if s.points[i].hash != s.points[j].hash {
            i++
            s.points[i] = s.points[j]
        }
    }
}

// findPoint finds point position for specified key.
func (s *StrategyConsistentHashing) findPoint(key string) int {
    i := 0
    j := len(s.points)
    hash := crc32.ChecksumIEEE([]byte(key))

    for ; i < j; {
        k := (i+j) / 2
        if hash > s.points[k].hash {
            i = k + 1
        } else if hash < s.points[k].hash {
            j = k
        } else {
            return k
        }
    }

    return i
}

// getServers returns servers for specified key.
func (s *StrategyConsistentHashing) getServers(key string, count uint) []*UpstreamServer {
    servers := make([]*UpstreamServer, 0)
    point := s.findPoint(key)
    for i := uint(0); i < count; i++ {
        k := point % len(s.points)
        servers = append(servers, s.points[k].server)
        point++
    }

    return servers
}

// Next
func (s *StrategyConsistentHashing) Next(r *http.Request) (*UpstreamServer, error) {
    key, err := s.GetKey(r)
    if err != nil {
        return nil, fmt.Errorf("can't get hashing key: %w", err)
    }

    var next *UpstreamServer
    servers := s.getServers(key, s.BackupCount)
    for i := range servers {
        srv := servers[i]
        if srv.online {
            next = srv
            break
        }
    }

    if next == nil {
        err = errors.New("no valid servers")
    }

    return next, err
}
