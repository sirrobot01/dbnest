package database

import (
	"fmt"
	"sort"
	"sync"
)

var (
	enginesMu sync.RWMutex
	engines   = make(map[string]Engine)
)

// RegisterEngine registers a database engine
func RegisterEngine(engine Engine) {
	enginesMu.Lock()
	defer enginesMu.Unlock()
	engines[engine.Type()] = engine
}

// GetEngine returns a registered engine by type
func GetEngine(engineType string) (Engine, error) {
	enginesMu.RLock()
	defer enginesMu.RUnlock()
	
	engine, ok := engines[engineType]
	if !ok {
		return nil, fmt.Errorf("unknown engine type: %s", engineType)
	}
	return engine, nil
}

// ListEngines returns all available engine types
func ListEngines() []string {
	enginesMu.RLock()
	defer enginesMu.RUnlock()
	
	types := make([]string, 0, len(engines))
	for t := range engines {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}

// GetEngineInfo returns metadata about all registered engines
func GetEngineInfo() []map[string]interface{} {
	enginesMu.RLock()
	defer enginesMu.RUnlock()
	
	info := make([]map[string]interface{}, 0, len(engines))
	for _, engine := range engines {
		info = append(info, map[string]interface{}{
			"type":        engine.Type(),
			"name":        engine.Name(),
			"defaultPort": engine.DefaultPort(),
			"versions":    engine.Versions(),
		})
	}
	return info
}
