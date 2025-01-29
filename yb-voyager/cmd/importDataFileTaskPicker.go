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

	"github.com/mroth/weightedrand/v2"
	"github.com/samber/lo"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/rand"
)

const (
	SHARDED   = "sharded"
	COLOCATED = "colocated"
)

type FileTaskPicker interface {
	NextTask() (*ImportFileTask, error)
	MarkTaskAsDone(task *ImportFileTask) error
	HasMoreTasks() bool
}

/*
A sequential task picker ensures that mulitple tasks are not being processed at the same time.
It will always pick the same task (first task in the pending list) until it is marked as done.
*/
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
	// it is assumed that the task is in pendingTasks and the first task in the list.
	// because SequentialTaskPicker will always pick the first task from the list.
	if !s.HasMoreTasks() {
		return fmt.Errorf("no more pending tasks to mark as done")
	}
	if s.pendingTasks[0].ID != task.ID {
		return fmt.Errorf("Task provided is not the first pending task. task's id = %d, first pending task's id = %d. ", task.ID, s.pendingTasks[0].ID)
	}
	s.pendingTasks = s.pendingTasks[1:]
	s.doneTasks = append(s.doneTasks, task)
	return nil
}

func (s *SequentialTaskPicker) HasMoreTasks() bool {
	return len(s.pendingTasks) > 0
}

/*
Pick any table at random, but tables have probabilities.
All sharded tables have equal probabilities.
The probabilities of all colocated tables sum up to the probability of a single sharded table.

	In essence, since all colocated tables are in a single tablet, they are all considered
	to represent one single table from the perspective of picking.

	If there are 4 colocated tables and 4 sharded tables,
		the probabilities for each of the sharded tables will be 1/5 (4 sharded + 1 for colocated)  = 0.2.
		The probabilities of each of the colocated tables will be 0.2/4 = 0.05.
		Therefore, (0.05 x 4) + (0.2 x 4) = 1

At any given time, only X  distinct tables can be IN-PROGRESS. If X=4, after picking four distinct tables,

	we will not pick a new 5th table. Only when one of the four is completely imported,
	we can go on to pick a different table. This is just to make it slightly easier from a
	status reporting/debugging perspective.
	X  >= N (no. of nodes in the cluster). This will lead to a better chance of achieving even load on the cluster.
*/
type ColocatedAwareRandomTaskPicker struct {
	// pendingTasks    []*ImportFileTask
	doneTasks             []*ImportFileTask
	inProgressTasks       []*ImportFileTask
	tableWisePendingTasks *utils.StructMap[sqlname.NameTuple, []*ImportFileTask]
	maxTasksInProgress    int

	tableTypes *utils.StructMap[sqlname.NameTuple, string] //colocated or sharded

	tableChooser *weightedrand.Chooser[sqlname.NameTuple, int]
}

type YbTargetDBColocatedChecker interface {
	IsDBColocated() (bool, error)
	IsTableColocated(tableName sqlname.NameTuple) (bool, error)
}

func NewColocatedAwareRandomTaskPicker(maxTasksInProgress int, tasks []*ImportFileTask, state *ImportDataState, yb YbTargetDBColocatedChecker) (*ColocatedAwareRandomTaskPicker, error) {
	// var pendingTasks []*ImportFileTask
	var doneTasks []*ImportFileTask
	var inProgressTasks []*ImportFileTask
	tableWisePendingTasks := utils.NewStructMap[sqlname.NameTuple, []*ImportFileTask]()
	tableTypes := utils.NewStructMap[sqlname.NameTuple, string]()

	isDBColocated, err := yb.IsDBColocated()
	if err != nil {
		return nil, fmt.Errorf("checking if db is colocated: %w", err)
	}

	for _, task := range tasks {
		tableName := task.TableNameTup

		// set tableType if not already set
		if _, ok := tableTypes.Get(tableName); !ok {
			var tableType string
			if !isDBColocated {
				tableType = SHARDED
			} else {
				isColocated, err := yb.IsTableColocated(tableName)
				if err != nil {
					return nil, fmt.Errorf("checking if table is colocated: table: %v: %w", tableName, err)
				}
				tableType = lo.Ternary(isColocated, COLOCATED, SHARDED)
			}
			tableTypes.Put(tableName, tableType)
		}

		// put task into right bucket.
		taskStatus, err := state.GetFileImportState(task.FilePath, task.TableNameTup)
		if err != nil {
			return nil, fmt.Errorf("getting file import state for tasl: %v: %w", task, err)
		}
		switch taskStatus {
		case FILE_IMPORT_COMPLETED:
			doneTasks = append(doneTasks, task)
		case FILE_IMPORT_IN_PROGRESS:
			inProgressTasks = append(inProgressTasks, task)
		case FILE_IMPORT_NOT_STARTED:
			// put into the table wise pending tasks.
			var tablePendingTasks []*ImportFileTask
			var ok bool
			tablePendingTasks, ok = tableWisePendingTasks.Get(tableName)
			if !ok {
				tablePendingTasks = []*ImportFileTask{}
			}
			tablePendingTasks = append(tablePendingTasks, task)
			tableWisePendingTasks.Put(tableName, tablePendingTasks)
		default:
			return nil, fmt.Errorf("unexpected  status for task: %v: %v", task, taskStatus)
		}
	}

	return &ColocatedAwareRandomTaskPicker{
		doneTasks:             doneTasks,
		inProgressTasks:       inProgressTasks,
		maxTasksInProgress:    maxTasksInProgress,
		tableWisePendingTasks: tableWisePendingTasks,
		tableTypes:            tableTypes,
	}, nil
}

func (c *ColocatedAwareRandomTaskPicker) NextTask() (*ImportFileTask, error) {
	if !c.HasMoreTasks() {
		return nil, fmt.Errorf("no more tasks")
	}

	if c.tableChooser == nil {
		c.initializeChooser()
	}

	// if we have already picked maxTasksInProgress tasks, pick a task from inProgressTasks
	if len(c.inProgressTasks) == c.maxTasksInProgress {
		return c.PickTaskFromInProgressTasks()
	}

	// pick a new task from pending tasks
	return c.PickTaskFromPendingTasks()
}

func (c *ColocatedAwareRandomTaskPicker) PickTaskFromInProgressTasks() (*ImportFileTask, error) {
	if len(c.inProgressTasks) == 0 {
		return nil, fmt.Errorf("no tasks in progress")
	}

	// pick a random task from inProgressTasks
	taskIndex := rand.Intn(len(c.inProgressTasks))
	return c.inProgressTasks[taskIndex], nil
}

func (c *ColocatedAwareRandomTaskPicker) PickTaskFromPendingTasks() (*ImportFileTask, error) {
	if len(c.tableWisePendingTasks.Keys()) == 0 {
		return nil, fmt.Errorf("no pending tasks to pick from")
	}
	tablePick := c.tableChooser.Pick()
	tablePendingTasks, ok := c.tableWisePendingTasks.Get(tablePick)
	if !ok {
		return nil, fmt.Errorf("no pending tasks for table picked: %s: %v", tablePick, c.tableWisePendingTasks)
	}

	pickedTask := tablePendingTasks[0]
	tablePendingTasks = tablePendingTasks[1:]

	if len(tablePendingTasks) == 0 {
		c.tableWisePendingTasks.Delete(tablePick)

		// reinitialize chooser because we have removed a table from the pending list, so weights will change.
		err := c.initializeChooser()
		if err != nil {
			return nil, fmt.Errorf("re-initializing chooser after picking task: %v: %w", pickedTask, err)
		}
	} else {
		c.tableWisePendingTasks.Put(tablePick, tablePendingTasks)
	}
	c.inProgressTasks = append(c.inProgressTasks, pickedTask)
	return pickedTask, nil
}

func (c *ColocatedAwareRandomTaskPicker) initializeChooser() error {
	tableNames := make([]sqlname.NameTuple, 0, len(c.tableWisePendingTasks.Keys()))
	c.tableWisePendingTasks.IterKV(func(k sqlname.NameTuple, v []*ImportFileTask) (bool, error) {
		tableNames = append(tableNames, k)
		return true, nil
	})

	colocatedCount := 0
	for _, tableName := range tableNames {
		tableType, ok := c.tableTypes.Get(tableName)
		if !ok {
			return fmt.Errorf("table type not found for table: %v", tableName)
		}

		if tableType == COLOCATED {
			colocatedCount++
		}
	}

	choices := []weightedrand.Choice[sqlname.NameTuple, int]{}
	for _, tableName := range tableNames {
		tableType, ok := c.tableTypes.Get(tableName)
		if !ok {
			return fmt.Errorf("table type not found for table: %v", tableName)
		}
		if tableType == COLOCATED {
			choices = append(choices, weightedrand.NewChoice(tableName, 1))
		} else {
			choices = append(choices, weightedrand.NewChoice(tableName, colocatedCount))
		}

		var err error
		c.tableChooser, err = weightedrand.NewChooser(choices...)
		if err != nil {
			return fmt.Errorf("creating chooser: %w", err)
		}
	}
	return nil
}

func (c *ColocatedAwareRandomTaskPicker) MarkTaskAsDone(task *ImportFileTask) error {
	for i, t := range c.inProgressTasks {
		if t.ID == task.ID {
			c.inProgressTasks = append(c.inProgressTasks[:i], c.inProgressTasks[i+1:]...)
			c.doneTasks = append(c.doneTasks, task)
			return nil
		}
	}
	return fmt.Errorf("task [%v] not found in inProgressTasks: %v", task, c.inProgressTasks)
}

func (c *ColocatedAwareRandomTaskPicker) HasMoreTasks() bool {
	if len(c.inProgressTasks) > 0 {
		return true
	}

	pendingTasks := false
	c.tableWisePendingTasks.IterKV(func(tableName sqlname.NameTuple, tasks []*ImportFileTask) (bool, error) {
		if len(tasks) > 0 {
			pendingTasks = true
			return false, nil
		}
		return true, nil
	})

	return pendingTasks
}
