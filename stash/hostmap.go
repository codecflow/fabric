package stash

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type HostMap struct {
	hosts       map[string]*Host
	mu          sync.RWMutex
	onHostAdded func(*Host) error
}

func NewHostMap(onHostAdded func(*Host) error) *HostMap {
	return &HostMap{
		hosts:       make(map[string]*Host),
		mu:          sync.RWMutex{},
		onHostAdded: onHostAdded,
	}
}

func (hm *HostMap) Set(host *Host) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	_, exists := hm.hosts[host.ID]
	if exists {
		return
	}

	hm.hosts[host.ID] = host
	if hm.onHostAdded != nil {
		slog.Info("Added new host @ %s (PrivateAddr=%s, RTT=%s)", host.ID, host.Address.Private, host.RTT)
		hm.onHostAdded(host)
	}
}

func (hm *HostMap) Remove(host *Host) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.hosts[host.ID]; exists {
		slog.Info("Removed host @ %s (PrivateAddr=%s, RTT=%s)", host.ID, host.Address.Private, host.RTT)
		delete(hm.hosts, host.ID)
	}
}

func (hm *HostMap) Len() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.hosts)
}

func (hm *HostMap) All() []*Host {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	hosts := make([]*Host, 0, len(hm.hosts))

	for _, host := range hm.hosts {
		hosts = append(hosts, host)
	}

	return hosts
}

func (hm *HostMap) Get(id string) *Host {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if host, exists := hm.hosts[id]; exists {
		return host
	}

	return nil
}

func (hm *HostMap) ClosestWithCapacity(ctx context.Context) (*Host, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		hosts := hm.All()

		if len(hosts) > 0 {
			sort.Slice(hosts, func(i, j int) bool {
				if hosts[i].RTT != hosts[j].RTT {
					return hosts[i].RTT < hosts[j].RTT
				}
				return hosts[i].Capacity.Used < hosts[j].Capacity.Used
			})

			return hosts[0], nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (hm *HostMap) Closest(ctx context.Context) (*Host, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		hosts := hm.All()

		if len(hosts) > 0 {
			sort.Slice(hosts, func(i, j int) bool { return hosts[i].RTT < hosts[j].RTT })

			return hosts[0], nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
