package httpsession

import (
	"sync"
	"time"
)

var _ Store = NewMemoryStore(30)

type sessionNode struct {
	lock   sync.RWMutex
	kvs    map[string]interface{}
	last   time.Time
	maxAge time.Duration
}

func (node *sessionNode) Get(key string) interface{} {
	node.lock.RLock()
	v := node.kvs[key]
	node.lock.RUnlock()
	node.lock.Lock()
	node.last = time.Now()
	node.lock.Unlock()
	return v
}

func (node *sessionNode) Set(key string, v interface{}) {
	node.lock.Lock()
	node.kvs[key] = v
	node.last = time.Now()
	node.lock.Unlock()
}

func (node *sessionNode) Del(key string) {
	node.lock.Lock()
	delete(node.kvs, key)
	node.last = time.Now()
	node.lock.Unlock()
}

type MemoryStore struct {
	lock       sync.RWMutex
	nodes      map[Id]*sessionNode
	GcInterval time.Duration
	maxAge     time.Duration
}

func NewMemoryStore(maxAge time.Duration) *MemoryStore {
	return &MemoryStore{nodes: make(map[Id]*sessionNode),
		maxAge: maxAge, GcInterval: 10 * time.Second}
}

func (store *MemoryStore) SetMaxAge(maxAge time.Duration) {
	store.lock.Lock()
	store.maxAge = maxAge
	store.lock.Unlock()
}

func (store *MemoryStore) Get(id Id, key string) interface{} {
	store.lock.RLock()
	node, ok := store.nodes[id]
	store.lock.RUnlock()
	if !ok {
		return nil
	}

	if store.maxAge > 0 && time.Now().Sub(node.last) > node.maxAge {
		// lazy DELETE expire
		store.lock.Lock()
		delete(store.nodes, id)
		store.lock.Unlock()
		return nil
	}

	return node.Get(key)
}

func (store *MemoryStore) Set(id Id, key string, value interface{}) {
	store.lock.RLock()
	node, ok := store.nodes[id]
	store.lock.RUnlock()
	if !ok {
		store.lock.Lock()
		node = store.newNode()
		node.kvs[key] = value
		store.nodes[id] = node
		store.lock.Unlock()
	}

	node.Set(key, value)
}

func (store *MemoryStore) newNode() *sessionNode {
	return &sessionNode{
		kvs: make(map[string]interface{}), 
		last: time.Now(),
		maxAge: store.maxAge,
	}
}

func (store *MemoryStore) Add(id Id) {
	node := store.newNode()
	store.lock.Lock()
	store.nodes[id] = node
	store.lock.Unlock()
}

func (store *MemoryStore) Del(id Id, key string) bool {
	store.lock.RLock()
	node, ok := store.nodes[id]
	store.lock.RUnlock()
	if ok {
		node.Del(key)
	}
	return true
}

func (store *MemoryStore) Exist(id Id) bool {
	store.lock.RLock()
	defer store.lock.RUnlock()
	_, ok := store.nodes[id]
	return ok
}

func (store *MemoryStore) Clear(id Id) bool {
	store.lock.Lock()
	defer store.lock.Unlock()
	delete(store.nodes, id)
	return true
}

func (store *MemoryStore) Run() error {
	time.AfterFunc(store.GcInterval, func() {
		store.Run()
		store.GC()
	})
	return nil
}

//
func (store *MemoryStore) GC() {
	store.lock.Lock()
	defer store.lock.Unlock()
	if store.maxAge == 0 {
		return
	}
	var i, j int
	for k, v := range store.nodes {
		if j > 20 || i > 5 {
			break
		}
		if time.Now().Sub(v.last) > v.maxAge {
			delete(store.nodes, k)
			i = i + 1
		}
		j = j + 1
	}

}
