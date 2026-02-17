package scan

import "sync"

var (
	registry   = make(map[string]Plugin)
	bySubject  = make(map[string]Plugin)
	registryMu sync.RWMutex
)

// Register registers a plugin by kind and subject.
func Register(p Plugin) {
	registryMu.Lock()
	defer registryMu.Unlock()
	d := p.Descriptor()
	registry[d.Kind] = p
	bySubject[d.Subject] = p
}

// Get returns the plugin for the given kind, or nil.
func Get(kind string) Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[kind]
}

// GetBySubject returns the plugin for the given NATS subject, or nil.
func GetBySubject(subject string) Plugin {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return bySubject[subject]
}

// Kinds returns all registered kinds.
func Kinds() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	kinds := make([]string, 0, len(registry))
	for k := range registry {
		kinds = append(kinds, k)
	}
	return kinds
}
