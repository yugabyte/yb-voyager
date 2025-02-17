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

	"github.com/davecgh/go-spew/spew"
	"github.com/mroth/weightedrand/v2"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/rand"
)

const (
	SHARDED   = "sharded"
	COLOCATED = "colocated"
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
	During this time, each of the in-progress tables will be picked with equal probability.
*/
type ColocatedAwareRandomTaskPicker struct {
	doneTasks []*ImportFileTask
	// tasks which the picker has picked at least once, and are essentially in progress.
	// the length of this list will be <= maxTasksInProgress
	inProgressTasks    []*ImportFileTask
	maxTasksInProgress int

	// tasks which have not yet been picked even once.
	// the tableChooser will be employed to pick a table from this list.
	tableWisePendingTasks *utils.StructMap[sqlname.NameTuple, []*ImportFileTask]
	tableChooser          *weightedrand.Chooser[sqlname.NameTuple, int]

	tableTypes *utils.StructMap[sqlname.NameTuple, string] //colocated or sharded

	state *ImportDataState
}

type YbTargetDBColocatedChecker interface {
	IsDBColocated() (bool, error)
	IsTableColocated(tableName sqlname.NameTuple) (bool, error)
}

func NewColocatedAwareRandomTaskPicker(maxTasksInProgress int, tasks []*ImportFileTask, state *ImportDataState, yb YbTargetDBColocatedChecker) (*ColocatedAwareRandomTaskPicker, error) {
	var doneTasks []*ImportFileTask
	var inProgressTasks []*ImportFileTask
	tableWisePendingTasks := utils.NewStructMap[sqlname.NameTuple, []*ImportFileTask]()
	tableTypes := utils.NewStructMap[sqlname.NameTuple, string]()

	isDBColocated, err := yb.IsDBColocated()
	if err != nil {
		return nil, fmt.Errorf("checking if db is colocated: %w", err)
	}

	addToPendingTasks := func(t *ImportFileTask) {
		// put into the table wise pending tasks.
		var tablePendingTasks []*ImportFileTask
		var ok bool
		tablePendingTasks, ok = tableWisePendingTasks.Get(t.TableNameTup)
		if !ok {
			tablePendingTasks = []*ImportFileTask{}
		}
		tablePendingTasks = append(tablePendingTasks, t)
		tableWisePendingTasks.Put(t.TableNameTup, tablePendingTasks)
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
			if len(inProgressTasks) < maxTasksInProgress {
				inProgressTasks = append(inProgressTasks, task)
			} else {
				addToPendingTasks(task)
			}
		case FILE_IMPORT_NOT_STARTED:
			addToPendingTasks(task)
		default:
			return nil, fmt.Errorf("unexpected  status for task: %v: %v", task, taskStatus)
		}
	}

	picker := &ColocatedAwareRandomTaskPicker{
		doneTasks:             doneTasks,
		inProgressTasks:       inProgressTasks,
		maxTasksInProgress:    maxTasksInProgress,
		tableWisePendingTasks: tableWisePendingTasks,
		tableTypes:            tableTypes,
		state:                 state,
	}
	err = picker.initializeChooser()
	if err != nil {
		return nil, fmt.Errorf("initializing chooser: %w", err)
	}

	log.Infof("ColocatedAwareRandomTaskPicker initialized with params:%v", spew.Sdump(picker))
	return picker, nil
}

func (c *ColocatedAwareRandomTaskPicker) Pick() (*ImportFileTask, error) {
	if !c.HasMoreTasks() {
		return nil, fmt.Errorf("no more tasks")
	}
	var task *ImportFileTask
	var err error
	// if we have already picked maxTasksInProgress tasks, pick a task from inProgressTasks
	if len(c.inProgressTasks) == c.maxTasksInProgress {
		task, err = c.PickTaskFromInProgressTasks()
	}

	// if we have less than maxTasksInProgress tasks in progress, but no pending tasks, pick a task from inProgressTasks
	if len(c.inProgressTasks) < c.maxTasksInProgress && len(c.tableWisePendingTasks.Keys()) == 0 {
		task, err = c.PickTaskFromInProgressTasks()
	}

	// pick a new task from pending tasks
	task, err = c.PickTaskFromPendingTasks()
	c.reportStateOfInProgressTasks()
	return task, err
}

func (c *ColocatedAwareRandomTaskPicker) reportStateOfInProgressTasks() {
	var inProgressColocatedCount int
	var inProgressShardedCount int
	var workerPoolColocatedBatchesCount int
	var workerPoolShardedBatchesCount int

	for _, task := range c.inProgressTasks {
		tableType, ok := c.tableTypes.Get(task.TableNameTup)
		if !ok {
			panic(fmt.Sprintf("table type not found for task: %v", task))
		}
		pendingBatches, err := c.state.GetPendingBatches(task.FilePath, task.TableNameTup)
		if err != nil {
			panic(fmt.Sprintf("getting pending batches for task: %v: %v", task, err))
		}
		if tableType == COLOCATED {
			inProgressColocatedCount++
			workerPoolColocatedBatchesCount += len(pendingBatches)
		} else {
			inProgressShardedCount++
			workerPoolShardedBatchesCount += len(pendingBatches)
		}
	}
	log.Infof("picker-pool state: In-Progress tasks: Colocated: %d, Sharded: %d. WorkerPool in-progress Batches: Colocated: %d, Sharded: %d",
		inProgressColocatedCount, inProgressShardedCount, workerPoolColocatedBatchesCount, workerPoolShardedBatchesCount)

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
	if c.tableChooser == nil {
		return nil, fmt.Errorf("chooser not initialized")
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
		if len(c.tableWisePendingTasks.Keys()) > 0 {
			err := c.initializeChooser()
			if err != nil {
				return nil, fmt.Errorf("re-initializing chooser after picking task: %v: %w", pickedTask, err)
			}
		}
	} else {
		c.tableWisePendingTasks.Put(tablePick, tablePendingTasks)
	}
	c.inProgressTasks = append(c.inProgressTasks, pickedTask)
	log.Infof("Picked task: %v. In-Progress tasks:%v", pickedTask, c.inProgressTasks)
	return pickedTask, nil
}

func (c *ColocatedAwareRandomTaskPicker) initializeChooser() error {
	if len(c.tableWisePendingTasks.Keys()) == 0 {
		return fmt.Errorf("no pending tasks to initialize chooser")
	}
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
	colocatedWeight := 1
	// if all sharded tables, then equal weight of 1.
	// otherwise, weight of a sharded tables = weight of all colocated tables.
	shardedWeight := lo.Ternary(colocatedCount == 0, 1, colocatedCount*colocatedWeight)

	choices := []weightedrand.Choice[sqlname.NameTuple, int]{}
	for _, tableName := range tableNames {
		tableType, ok := c.tableTypes.Get(tableName)
		if !ok {
			return fmt.Errorf("table type not found for table: %v", tableName)
		}
		if tableType == COLOCATED {
			choices = append(choices, weightedrand.NewChoice(tableName, colocatedWeight))
		} else {
			choices = append(choices, weightedrand.NewChoice(tableName, shardedWeight))
		}

	}
	var err error
	c.tableChooser, err = weightedrand.NewChooser(choices...)
	if err != nil {
		return fmt.Errorf("creating chooser: %w", err)
	}
	return nil
}

func (c *ColocatedAwareRandomTaskPicker) MarkTaskAsDone(task *ImportFileTask) error {
	for i, t := range c.inProgressTasks {
		if t.ID == task.ID {
			c.inProgressTasks = append(c.inProgressTasks[:i], c.inProgressTasks[i+1:]...)
			c.doneTasks = append(c.doneTasks, task)
			log.Infof("Marked task as done: %v. In-Progress tasks:%v", t, c.inProgressTasks)
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

func (c *ColocatedAwareRandomTaskPicker) WaitForTasksBatchesTobeImported() {
	// no wait
	return
}
