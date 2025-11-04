package executor

import (
	"fmt"
	"sync"

	"github.com/lureiny/lookingglass/agent/config"
)

// ExecutorFactory is a function that creates an executor instance
type ExecutorFactory func(cfg *config.TaskConfig) (Executor, error)

// Registry manages executor factories
type Registry struct {
	factories map[string]ExecutorFactory
	mutex     sync.RWMutex
}

// NewRegistry creates a new executor registry
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ExecutorFactory),
	}
}

// Register registers an executor factory with an executor type
func (r *Registry) Register(executorType string, factory ExecutorFactory) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if executorType == "" {
		return fmt.Errorf("executor type cannot be empty")
	}

	if factory == nil {
		return fmt.Errorf("factory function cannot be nil")
	}

	r.factories[executorType] = factory
	return nil
}

// Create creates an executor instance using the registered factory
func (r *Registry) Create(executorType string, cfg *config.TaskConfig) (Executor, error) {
	r.mutex.RLock()
	factory, ok := r.factories[executorType]
	r.mutex.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no factory registered for executor type: %s", executorType)
	}

	return factory(cfg)
}

// HasExecutor checks if an executor type is registered
func (r *Registry) HasExecutor(executorType string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, ok := r.factories[executorType]
	return ok
}

// GetRegisteredTypes returns all registered executor types
func (r *Registry) GetRegisteredTypes() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	types := make([]string, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}

	return types
}

// Global registry instance
var globalRegistry = NewRegistry()

// RegisterGlobal registers an executor factory to the global registry
func RegisterGlobal(executorType string, factory ExecutorFactory) error {
	return globalRegistry.Register(executorType, factory)
}

// CreateGlobal creates an executor using the global registry
func CreateGlobal(executorType string, cfg *config.TaskConfig) (Executor, error) {
	return globalRegistry.Create(executorType, cfg)
}

// GetGlobalRegistry returns the global registry instance
func GetGlobalRegistry() *Registry {
	return globalRegistry
}
