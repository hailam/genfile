// internal/adapters/factory/generator_factory.go
package factory

import (
	"fmt"
	"log"
	"sync"

	"github.com/hailam/genfile/internal/ports"
)

// registry stores the registered generators.
var (
	generatorRegistry = make(map[ports.FileType]ports.FileGenerator)
	registryMutex     sync.RWMutex
)

// RegisterGenerator is called by generator packages during their init() phase.
func RegisterGenerator(fileType ports.FileType, generator ports.FileGenerator) {
	registryMutex.Lock()
	defer registryMutex.Unlock()
	if _, exists := generatorRegistry[fileType]; exists {
		log.Printf("Warning: Duplicate generator registration for %s. Overwriting existing one.", fileType)
	}
	generatorRegistry[fileType] = generator
	// fmt.Printf("factory: Registered generator for %s\n", fileType)
}

// DynamicGeneratorFactory uses the registry populated by RegisterGenerator.
type DynamicGeneratorFactory struct{}

// NewGeneratorFactory creates a new factory that uses the global registry.
func NewGeneratorFactory() ports.GeneratorFactory {
	return &DynamicGeneratorFactory{}
}

// For returns the appropriate FileGenerator for the given FileType from the registry.
func (f *DynamicGeneratorFactory) For(t ports.FileType) (ports.FileGenerator, error) {
	registryMutex.RLock()
	defer registryMutex.RUnlock()

	gen, ok := generatorRegistry[t]
	if !ok {
		return nil, fmt.Errorf("unsupported file type: '%s' (no generator registered or check file extension)", t)
	}
	return gen, nil
}

func RegisteredTypes() []ports.FileType {
	registryMutex.RLock()
	defer registryMutex.RUnlock()
	types := make([]ports.FileType, 0, len(generatorRegistry))
	for t := range generatorRegistry {
		types = append(types, t)
	}
	return types
}
