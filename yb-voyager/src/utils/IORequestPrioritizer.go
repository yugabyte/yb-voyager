package utils

import "sync"

type IORequestPrioritizer struct {
	mu                sync.Mutex
	highPriorityCount int
	cond              *sync.Cond
}

var IRP *IORequestPrioritizer

func NewIORequestPrioritizer() *IORequestPrioritizer {
	irp := &IORequestPrioritizer{}
	return irp
}

func (irp *IORequestPrioritizer) RequestToRunHighPriorityIO() {
	irp.mu.Lock()
	irp.highPriorityCount++
	irp.mu.Unlock()
}

func (irp *IORequestPrioritizer) ReleaseHighPriorityIO() {
	irp.mu.Lock()
	irp.highPriorityCount--
	if irp.highPriorityCount == 0 {
		irp.cond.Broadcast()
	}
	irp.mu.Unlock()
}

func (irp *IORequestPrioritizer) RequestToRunLowPriorityIO() {
	// wait until high priority requests are released
	irp.mu.Lock()
	for irp.highPriorityCount > 0 {
		irp.cond.Wait()
	}
	irp.mu.Unlock()
}

func (irp *IORequestPrioritizer) ReleaseLowPriorityIO() {
	// no-op for now
}

func init() {
	IRP = NewIORequestPrioritizer()
}
