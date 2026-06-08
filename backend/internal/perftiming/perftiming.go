package perftiming

import (
	"sync"
	"time"
)

type Recorder interface {
	RecordStageTiming(traceID string, stage string, duration time.Duration)
	RecordDBStageTiming(traceID string, stage string, duration time.Duration)
}

var registry = struct {
	sync.RWMutex
	nextID    uint64
	recorders map[uint64]Recorder
}{
	recorders: make(map[uint64]Recorder),
}

func Register(recorder Recorder) func() {
	if recorder == nil {
		return func() {}
	}
	registry.Lock()
	registry.nextID++
	id := registry.nextID
	registry.recorders[id] = recorder
	registry.Unlock()
	return func() {
		registry.Lock()
		delete(registry.recorders, id)
		registry.Unlock()
	}
}

func RecordStageTiming(traceID string, stage string, duration time.Duration) {
	registry.RLock()
	recorders := make([]Recorder, 0, len(registry.recorders))
	for _, recorder := range registry.recorders {
		recorders = append(recorders, recorder)
	}
	registry.RUnlock()
	for _, recorder := range recorders {
		recorder.RecordStageTiming(traceID, stage, duration)
	}
}

func RecordDBStageTiming(traceID string, stage string, duration time.Duration) {
	registry.RLock()
	recorders := make([]Recorder, 0, len(registry.recorders))
	for _, recorder := range registry.recorders {
		recorders = append(recorders, recorder)
	}
	registry.RUnlock()
	for _, recorder := range recorders {
		recorder.RecordDBStageTiming(traceID, stage, duration)
	}
}
