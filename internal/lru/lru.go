package lru

import (
	"container/list"
)

// LRUCache is a type-safe LRU cache implementation using generics.
type Cache[K comparable, V any] struct {
	capacity int
	cache    map[K]*list.Element
	queue    *list.List
}

// entry holds the key and value. It makes use of generics for type safety.
type entry[K comparable, V any] struct {
	key   K
	value V
}

// NewLRUCache creates a new LRU cache with the specified capacity.
func NewLRUCache[K comparable, V any](capacity int) *Cache[K, V] {
	return &Cache[K, V]{
		capacity: capacity,
		cache:    make(map[K]*list.Element),
		queue:    list.New(),
	}
}

// Get retrieves a value from the cache. It returns the value and a boolean indicating if the key was found.
func (c *Cache[K, V]) Get(key K) (V, bool) { //nolint:ireturn // This is a generic function
	if elem, found := c.cache[key]; found {
		c.queue.MoveToFront(elem)

		if entry, ok := elem.Value.(*entry[K, V]); ok {
			return entry.value, true
		}
	}

	var zeroValue V // Return zero value of V if not found

	return zeroValue, false
}

// Set adds a key-value pair to the cache or updates an existing key. It also handles the eviction of the least recently used item if necessary.
func (c *Cache[K, V]) Set(key K, value V) {
	if elem, found := c.cache[key]; found {
		c.queue.MoveToFront(elem)

		if entry, ok := elem.Value.(*entry[K, V]); ok {
			entry.value = value
		}

		return
	}

	elem := c.queue.PushFront(&entry[K, V]{key, value})
	c.cache[key] = elem

	if c.queue.Len() > c.capacity {
		oldest := c.queue.Back()
		if oldest != nil {
			if entry, ok := oldest.Value.(*entry[K, V]); ok {
				c.queue.Remove(oldest)
				delete(c.cache, entry.key)
			}
		}
	}
}
