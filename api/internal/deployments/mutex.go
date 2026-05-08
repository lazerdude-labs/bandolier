package deployments

import "sync"

type ClusterMutex struct {
	mu  sync.Mutex
	per map[string]*sync.Mutex
}

func NewClusterMutex() *ClusterMutex {
	return &ClusterMutex{per: map[string]*sync.Mutex{}}
}

func (c *ClusterMutex) TryLock(clusterID string) bool {
	c.mu.Lock()
	m, ok := c.per[clusterID]
	if !ok {
		m = &sync.Mutex{}
		c.per[clusterID] = m
	}
	c.mu.Unlock()
	return m.TryLock()
}

func (c *ClusterMutex) Unlock(clusterID string) {
	c.mu.Lock()
	m, ok := c.per[clusterID]
	c.mu.Unlock()
	if ok {
		m.Unlock()
	}
}
