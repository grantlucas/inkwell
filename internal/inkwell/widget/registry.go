package widget

import (
	"fmt"
	"image"
	"time"
)

// Deps provides injectable dependencies for widget factories.
type Deps struct {
	Now         func() time.Time
	DataSources map[string]any
}

// Factory creates a Widget from bounds, a raw config map, and dependencies.
type Factory func(bounds image.Rectangle, config map[string]any, deps Deps) (Widget, error)

// Registry maps widget type names to their factory functions.
type Registry struct {
	factories map[string]Factory
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[string]Factory)}
}

// Register adds a widget factory under the given type name.
// It panics if typeName is empty, f is nil, or typeName is already registered.
func (r *Registry) Register(typeName string, f Factory) {
	if typeName == "" {
		panic("widget type name must not be empty")
	}
	if f == nil {
		panic(fmt.Sprintf("widget type %q has nil factory", typeName))
	}
	if _, exists := r.factories[typeName]; exists {
		panic(fmt.Sprintf("widget type %q already registered", typeName))
	}
	r.factories[typeName] = f
}

// Create instantiates a widget by type name.
func (r *Registry) Create(typeName string, bounds image.Rectangle, config map[string]any, deps Deps) (Widget, error) {
	f, ok := r.factories[typeName]
	if !ok {
		return nil, fmt.Errorf("unknown widget type: %q", typeName)
	}
	return f(bounds, config, deps)
}
