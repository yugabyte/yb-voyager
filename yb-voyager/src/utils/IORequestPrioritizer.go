package utils

import "sync"

type IORequestPrioritizer struct {
	highPriorityWaitGroup sync.WaitGroup
}

var IRP *IORequestPrioritizer

func NewIORequestPrioritizer() *IORequestPrioritizer {
	irp := &IORequestPrioritizer{}
	return irp
}

func (irp *IORequestPrioritizer) RequestToRunHighPriorityIO() {
	irp.highPriorityWaitGroup.Add(1)
}

func (irp *IORequestPrioritizer) ReleaseHighPriorityIO() {
	irp.highPriorityWaitGroup.Done()
}

func (irp *IORequestPrioritizer) RequestToRunLowPriorityIO() {
	// wait until high priority requests are released
	irp.highPriorityWaitGroup.Wait()
}

func (irp *IORequestPrioritizer) ReleaseLowPriorityIO() {
	// no-op for now
}

func init() {
	IRP = NewIORequestPrioritizer()
}
