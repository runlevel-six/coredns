package xds

import (
	"math/rand"
	"net"
	"sync"

	xdspb "github.com/envoyproxy/go-control-plane/envoy/api/v2"
)

type assignment struct {
	mu      sync.RWMutex
	cla     map[string]*xdspb.ClusterLoadAssignment
	version int // not sure what do with and if we should discard all clusters.
}

func (a *assignment) setClusterLoadAssignment(cluster string, cla *xdspb.ClusterLoadAssignment) {
	// If cla is nil we just found a cluster, check if we already know about it, or if we need to make a new entry.
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.cla[cluster]
	if !ok {
		a.cla[cluster] = cla
		return
	}
	if cla == nil {
		return
	}
	a.cla[cluster] = cla

}

func (a *assignment) clusterLoadAssignment(cluster string) *xdspb.ClusterLoadAssignment {
	a.mu.RLock()
	cla, ok := a.cla[cluster]
	a.mu.RUnlock()
	if !ok {
		return nil
	}
	return cla
}

func (a *assignment) clusters() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	clusters := make([]string, len(a.cla))
	i := 0
	for k := range a.cla {
		clusters[i] = k
		i++
	}
	return clusters
}

// Select selects a backend from cla, using weighted random selection. It only selects
// backends that are reporting healthy.
func (a *assignment) Select(cluster string) (net.IP, bool) {
	cla := a.clusterLoadAssignment(cluster)
	if cla == nil {
		return nil, false
	}

	total := 0
	i := 0
	for _, ep := range cla.Endpoints {
		for _, lb := range ep.GetLbEndpoints() {
			//			if lb.GetHealthStatus() != corepb.HealthStatus_HEALTHY {
			//				continue
			//			}
			total += int(lb.GetLoadBalancingWeight().GetValue())
			i++
		}
	}
	if total == 0 {
		// all weights are 0, randomly select one of the endpoints.
		r := rand.Intn(i)
		i := 0
		for _, ep := range cla.Endpoints {
			for _, lb := range ep.GetLbEndpoints() {
				//				if lb.GetHealthStatus() != corepb.HealthStatus_HEALTHY {
				//					continue
				//				}
				if r == i {
					return net.ParseIP(lb.GetEndpoint().GetAddress().GetSocketAddress().GetAddress()), true
				}
				i++
			}
		}
		return nil
	}

	r := rand.Intn(total) + 1

	for _, ep := range cla.Endpoints {
		for _, lb := range ep.GetLbEndpoints() {
			//			if lb.GetHealthStatus() != corepb.HealthStatus_HEALTHY {
			//				continue
			//			}
			r -= int(lb.GetLoadBalancingWeight().GetValue())
			if r <= 0 {
				return net.ParseIP(lb.GetEndpoint().GetAddress().GetSocketAddress().GetAddress()), true
			}
		}
	}
	return nil, false
}
