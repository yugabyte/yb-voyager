package utils

import (
	"sync"
)

type IORequestPrioritizer struct {
	mu                sync.Mutex
	highPriorityCount int
	lowPriorityCount  int
	cond              *sync.Cond
}

var IRP *IORequestPrioritizer

func NewIORequestPrioritizer() *IORequestPrioritizer {
	irp := &IORequestPrioritizer{}
	irp.cond = sync.NewCond(&irp.mu)
	return irp
}

func (irp *IORequestPrioritizer) RequestToRunHighPriorityIO() {
	// irp.mu.Lock()
	// irp.highPriorityCount++
	// log.Infof("Request to run high priority IO, highPriorityCount: %d, lowPriorityCount: %d\n", irp.highPriorityCount, irp.lowPriorityCount)
	// irp.mu.Unlock()
}

func (irp *IORequestPrioritizer) ReleaseHighPriorityIO() {
	// irp.mu.Lock()
	// irp.highPriorityCount--
	// if irp.highPriorityCount == 0 {
	// 	irp.cond.Broadcast()
	// }
	// irp.mu.Unlock()
}

func (irp *IORequestPrioritizer) RequestToRunLowPriorityIO() {
	// // wait until high priority requests are released
	// irp.mu.Lock()
	// for irp.highPriorityCount > 0 {
	// 	irp.cond.Wait()
	// }
	// irp.lowPriorityCount++
	// irp.mu.Unlock()
}

func (irp *IORequestPrioritizer) ReleaseLowPriorityIO() {
	// // no-op for now
	// irp.mu.Lock()
	// irp.lowPriorityCount--
	// irp.mu.Unlock()
}

func init() {
	IRP = NewIORequestPrioritizer()
}
