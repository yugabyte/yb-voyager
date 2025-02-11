/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"time"
)

type FileTaskPicker interface {
	Pick() (*ImportFileTask, error)
	MarkTaskAsDone(task *ImportFileTask) error
	HasMoreTasks() bool
	WaitForTasksBatchesTobeImported()
}

/*
A sequential task picker ensures that mulitple tasks are not being processed at the same time.
It will always pick the same task (first task in the pending list) until it is marked as done.
*/
type SequentialTaskPicker struct {
	inProgressTask *ImportFileTask
	pendingTasks   []*ImportFileTask
	doneTasks      []*ImportFileTask
}

func NewSequentialTaskPicker(tasks []*ImportFileTask, state *ImportDataState) (*SequentialTaskPicker, error) {
	var pendingTasks []*ImportFileTask
	var doneTasks []*ImportFileTask
	var inProgressTask *ImportFileTask
	for _, task := range tasks {
		taskStatus, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			return nil, fmt.Errorf("getting file import state for tasl: %v: %w", task, err)
		}
		switch taskStatus {
		case FILE_IMPORT_COMPLETED:
			doneTasks = append(doneTasks, task)
		case FILE_IMPORT_NOT_STARTED:
			pendingTasks = append(pendingTasks, task)
		case FILE_IMPORT_IN_PROGRESS:
			if inProgressTask != nil {
				return nil, fmt.Errorf("multiple tasks are in progress. task1: %v, task2: %v", inProgressTask, task)
			}
			inProgressTask = task
		default:
			return nil, fmt.Errorf("unexpected  status for task: %v: %v", task, taskStatus)
		}
	}
	return &SequentialTaskPicker{
		pendingTasks:   pendingTasks,
		doneTasks:      doneTasks,
		inProgressTask: inProgressTask,
	}, nil
}

func (s *SequentialTaskPicker) Pick() (*ImportFileTask, error) {
	if !s.HasMoreTasks() {
		return nil, fmt.Errorf("no more tasks")
	}

	if s.inProgressTask == nil {
		s.inProgressTask = s.pendingTasks[0]
		s.pendingTasks = s.pendingTasks[1:]
	}
	return s.inProgressTask, nil
}

func (s *SequentialTaskPicker) MarkTaskAsDone(task *ImportFileTask) error {

	if s.inProgressTask == nil {
		return fmt.Errorf("no task in progress to mark as done")
	}
	if s.inProgressTask.ID != task.ID {
		return fmt.Errorf("Task provided is not the task in progress. task's id = %d, task in progress's id = %d. ", task.ID, s.inProgressTask.ID)
	}
	s.inProgressTask = nil
	s.doneTasks = append(s.doneTasks, task)

	return nil
}

func (s *SequentialTaskPicker) HasMoreTasks() bool {
	if s.inProgressTask != nil {
		return true
	}
	return len(s.pendingTasks) > 0
}

func (s *SequentialTaskPicker) WaitForTasksBatchesTobeImported() {
	// Consider the scenario where we have a single task in progress and all batches are submitted, but not yet ingested.
	// In this case as per SequentialTaskPicker's implementation, it will wait for the task to be marked as done.
	// Instead of having a busy-loop where we keep checking if the task is done, we can wait for a second and then check again.
	time.Sleep(time.Second * 1)
}
