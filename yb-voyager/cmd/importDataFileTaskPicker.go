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
)

type FileTaskPicker interface {
	NextTask() (*ImportFileTask, error)
	MarkTaskAsDone(task *ImportFileTask) error
	HasMoreTasks() bool
}

type SequentialTaskPicker struct {
	pendingTasks []*ImportFileTask
	doneTasks    []*ImportFileTask
}

func NewSequentialTaskPicker(tasks []*ImportFileTask, state *ImportDataState) (*SequentialTaskPicker, error) {
	var pendingTasks []*ImportFileTask
	var doneTasks []*ImportFileTask
	for _, task := range tasks {
		taskStatus, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			return nil, fmt.Errorf("getting file import state for tasl: %v: %w", task, err)
		}
		switch taskStatus {
		case FILE_IMPORT_COMPLETED:
			doneTasks = append(doneTasks, task)
		case FILE_IMPORT_NOT_STARTED, FILE_IMPORT_IN_PROGRESS:
			pendingTasks = append(pendingTasks, task)
		default:
			return nil, fmt.Errorf("unexpected  status for task: %v: %v", task, taskStatus)
		}
	}
	return &SequentialTaskPicker{
		pendingTasks: pendingTasks,
		doneTasks:    doneTasks,
	}, nil
}

func (s *SequentialTaskPicker) NextTask() (*ImportFileTask, error) {
	if !s.HasMoreTasks() {
		return nil, fmt.Errorf("no more tasks")
	}
	return s.pendingTasks[0], nil
}

func (s *SequentialTaskPicker) MarkTaskAsDone(task *ImportFileTask) error {
	for i, t := range s.pendingTasks {
		if t.ID == task.ID {
			s.pendingTasks = append(s.pendingTasks[:i], s.pendingTasks[i+1:]...)
			s.doneTasks = append(s.doneTasks, task)
			return nil
		}
	}
	return fmt.Errorf("task not found")
}

func (s *SequentialTaskPicker) HasMoreTasks() bool {
	return len(s.pendingTasks) > 0
}
