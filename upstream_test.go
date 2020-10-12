package proxy

import (
    "testing"
)

func TestUpstreamServer(t *testing.T) {
    addr := "http://127.0.0.1:8080"
    weight := uint8(1)
    server := NewUpstreamServer(addr, weight)

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
}

