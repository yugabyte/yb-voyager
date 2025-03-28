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
	"errors"
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
	WaitForTasksBatchesTobeImported() error
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

func (s *SequentialTaskPicker) WaitForTasksBatchesTobeImported() error {
	// Consider the scenario where we have a single task in progress and all batches are submitted, but not yet ingested.
	// In this case as per SequentialTaskPicker's implementation, it will wait for the task to be marked as done.
	// Instead of having a busy-loop where we keep checking if the task is done, we can wait for a second and then check again.
	time.Sleep(time.Second * 1)
	return nil
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
	if len(picker.tableWisePendingTasks.Keys()) > 0 {
		err = picker.initializeChooser()
		if err != nil {
			return nil, fmt.Errorf("initializing chooser: %w", err)
		}
	}

	log.Infof("ColocatedAwareRandomTaskPicker initialized with params:%v", spew.Sdump(picker))
	return picker, nil
}

func (c *ColocatedAwareRandomTaskPicker) Pick() (*ImportFileTask, error) {
	if !c.HasMoreTasks() {
		return nil, fmt.Errorf("no more tasks")
	}

	// if we have already picked maxTasksInProgress tasks, pick a task from inProgressTasks
	if len(c.inProgressTasks) == c.maxTasksInProgress {
		return c.PickTaskFromInProgressTasks()
	}

	// if we have less than maxTasksInProgress tasks in progress, but no pending tasks, pick a task from inProgressTasks
	if len(c.inProgressTasks) < c.maxTasksInProgress && len(c.tableWisePendingTasks.Keys()) == 0 {
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
		// we need to update the weights every time the list of pending tasks change.
		// We can't simply remove a choice, because the weights need to be rebalanced based on what is pending.
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

func (c *ColocatedAwareRandomTaskPicker) WaitForTasksBatchesTobeImported() error {
	// if for all in-progress tasks, all batches are submitted, then sleep for a bit
	allTasksAllBatchesSubmitted := true

	for _, task := range c.inProgressTasks {
		taskAllBatchesSubmitted, err := c.state.AllBatchesSubmittedForTask(task.ID)
		if err != nil {
			return fmt.Errorf("checking if all batches are submitted for task: %v: %w", task, err)
		}
		if !taskAllBatchesSubmitted {
			allTasksAllBatchesSubmitted = false
			break
		}
	}

	if allTasksAllBatchesSubmitted {
		log.Infof("All batches submitted for all in-progress tasks. Sleeping")
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}

/*
The goal of this picker is to pick a combination of colocated and sharded tasks, both at random.
The limits in place are maxShardedTasksInProgress, maxColocatedTasksInProgress and colocatedBatchTaskQueue.

Colocated tasks are limited by single tablet performance limits on YB, so we have to constrain the no. of colocated
batches that can be ingested at a time. THis is achieved by having a max parallel limit on the consumer of colocatedBatchTaskQueue.
Therefore, colocated tasks are prioritized and picked but only if colocatedBatchTaskQueue has space.
If colocatedBatchTaskQueue is full, then sharded tasks are picked.
*/
type ColocatedCappedRandomTaskPicker struct {
	state *ImportDataState

	maxShardedTasksInProgress   int
	maxColocatedTasksInProgress int
	colocatedBatchTaskQueue     chan func()
	tableTypes                  *utils.StructMap[sqlname.NameTuple, string] //colocated or sharded

	doneTasks []*ImportFileTask
	// tasks which the picker has picked at least once, and are essentially in progress.
	// the length of this list will be <= maxTasksInProgress
	inProgressShardedTasks   []*ImportFileTask
	inProgressColocatedTasks []*ImportFileTask

	// tasks which have not yet been picked even once.
	pendingShardedTasks   []*ImportFileTask
	pendingColocatedTasks []*ImportFileTask
}

func NewColocatedCappedRandomTaskPicker(maxShardedTasksInProgress int, maxColocatedTasksInProgress int, tasks []*ImportFileTask,
	state *ImportDataState, yb YbTargetDBColocatedChecker, colocatedBatchTaskQueue chan func(),
	tableTypes *utils.StructMap[sqlname.NameTuple, string]) (*ColocatedCappedRandomTaskPicker, error) {
	var doneTasks []*ImportFileTask

	var inProgressColocatedTasks []*ImportFileTask
	var inProgressShardedTasks []*ImportFileTask
	var pendingColcatedTasks []*ImportFileTask
	var pendingShardedTasks []*ImportFileTask

	addToPendingTasks := func(t *ImportFileTask, tableType string) {
		if tableType == COLOCATED {
			pendingColcatedTasks = append(pendingColcatedTasks, t)
		} else {
			pendingShardedTasks = append(pendingShardedTasks, t)
		}
	}

	addToInProgressTasks := func(t *ImportFileTask, tableType string) {
		if tableType == COLOCATED {
			inProgressColocatedTasks = append(inProgressColocatedTasks, t)
		} else {
			inProgressShardedTasks = append(inProgressShardedTasks, t)
		}
	}

	for _, task := range tasks {
		tableName := task.TableNameTup
		tableType, ok := tableTypes.Get(tableName)
		if !ok {
			return nil, fmt.Errorf("table type not found for table: %v", tableName)
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
			switch tableType {
			case COLOCATED:
				if len(inProgressColocatedTasks) < maxColocatedTasksInProgress {
					addToInProgressTasks(task, tableType)
				} else {
					addToPendingTasks(task, tableType)
				}
			case SHARDED:
				if len(inProgressShardedTasks) < maxShardedTasksInProgress {
					addToInProgressTasks(task, tableType)
				} else {
					addToPendingTasks(task, tableType)
				}
			}
		case FILE_IMPORT_NOT_STARTED:
			addToPendingTasks(task, tableType)
		default:
			return nil, fmt.Errorf("unexpected  status for task: %v: %v", task, taskStatus)
		}
	}

	picker := &ColocatedCappedRandomTaskPicker{
		state:                       state,
		maxShardedTasksInProgress:   maxShardedTasksInProgress,
		maxColocatedTasksInProgress: maxColocatedTasksInProgress,

		doneTasks: doneTasks,

		inProgressColocatedTasks: inProgressColocatedTasks,
		inProgressShardedTasks:   inProgressShardedTasks,

		pendingColocatedTasks: pendingColcatedTasks,
		pendingShardedTasks:   pendingShardedTasks,

		tableTypes:              tableTypes,
		colocatedBatchTaskQueue: colocatedBatchTaskQueue,
	}

	log.Infof("ColocatedCappedRandomTaskPicker initialized with params:%v", spew.Sdump(picker))
	return picker, nil
}

func (c *ColocatedCappedRandomTaskPicker) inProgressTasks() []*ImportFileTask {
	return append(c.inProgressColocatedTasks, c.inProgressShardedTasks...)
}

func (c *ColocatedCappedRandomTaskPicker) pendingTasks() []*ImportFileTask {
	return append(c.pendingColocatedTasks, c.pendingShardedTasks...)
}

func (c *ColocatedCappedRandomTaskPicker) HasMoreTasks() bool {
	return len(c.inProgressTasks()) > 0 || len(c.pendingTasks()) > 0
}

func (c *ColocatedCappedRandomTaskPicker) HasMoreColocatedTasks() bool {
	return len(c.inProgressColocatedTasks) > 0 || len(c.pendingColocatedTasks) > 0
}

func (c *ColocatedCappedRandomTaskPicker) HasMoreShardedTasks() bool {
	return len(c.inProgressShardedTasks) > 0 || len(c.pendingShardedTasks) > 0
}

func (c *ColocatedCappedRandomTaskPicker) pickRandomFromListOfTasks(tasks []*ImportFileTask) (int, *ImportFileTask) {
	if len(tasks) == 0 {
		panic("no tasks to pick from")
	}
	// pick a random task
	taskIndex := rand.Intn(len(tasks))
	return taskIndex, tasks[taskIndex]
}

func (c *ColocatedCappedRandomTaskPicker) Pick() (*ImportFileTask, error) {
	if !c.HasMoreTasks() {
		return nil, fmt.Errorf("no more tasks")
	}

	// if only one type of tasks are left, pick from that type.
	if !c.HasMoreShardedTasks() {
		return c.pickColocatedTask()
	}
	if !c.HasMoreColocatedTasks() {
		return c.pickShardedTask()
	}

	// we have a combination of tasks left.
	// first fill up in-progress tasks from pending tasks if possible.
	task, err := c.pickPendingColocatedTaskAsPerMaxTasks()
	if err != nil {
		return nil, fmt.Errorf("picking pending colocated task: %w", err)
	}
	if task != nil {
		return task, nil
	}
	task, err = c.pickPendingShardedTaskAsPerMaxTasks()
	if err != nil {
		return nil, fmt.Errorf("picking pending sharded task: %w", err)
	}
	if task != nil {
		return task, nil
	}

	// pick from the combination of in-progress tasks.
	// if we can push a new colocated task into the queue, pick a colocated task.
	log.Debugf("colocatedBatchTaskQueue: %v, cap: %v", len(c.colocatedBatchTaskQueue), cap(c.colocatedBatchTaskQueue))
	if len(c.colocatedBatchTaskQueue) < cap(c.colocatedBatchTaskQueue) {
		return c.pickInProgressColocatedTask()
	}
	return c.pickInProgressShardedTask()
}

func (c *ColocatedCappedRandomTaskPicker) pickColocatedTask() (*ImportFileTask, error) {
	if !c.HasMoreColocatedTasks() {
		return nil, fmt.Errorf("no more colocated tasks")
	}

	// try to pick a colocated pending task.
	task, err := c.pickPendingColocatedTaskAsPerMaxTasks()
	if err != nil {
		return nil, fmt.Errorf("picking pending colocated task: %w", err)
	}
	if task != nil {
		return task, nil
	}

	// pick a colocated in-progress task.
	return c.pickInProgressColocatedTask()
}

func (c *ColocatedCappedRandomTaskPicker) pickPendingColocatedTaskAsPerMaxTasks() (*ImportFileTask, error) {
	if len(c.inProgressColocatedTasks) < c.maxColocatedTasksInProgress {
		if len(c.pendingColocatedTasks) > 0 {
			taskIndex, pickedTask := c.pickRandomFromListOfTasks(c.pendingColocatedTasks)
			c.pendingColocatedTasks = append(c.pendingColocatedTasks[:taskIndex], c.pendingColocatedTasks[taskIndex+1:]...)
			c.inProgressColocatedTasks = append(c.inProgressColocatedTasks, pickedTask)
			log.Debugf("picking pending colocated task: %v", pickedTask)
			return pickedTask, nil
		}
	}
	log.Debugf("could not pick pending colocated task. inProgressColocatedTasks: %v, maxColocatedTasksInProgress: %v, pendingColocatedTasks: %v",
		c.inProgressColocatedTasks, c.maxColocatedTasksInProgress, c.pendingColocatedTasks)
	return nil, nil
}

func (c *ColocatedCappedRandomTaskPicker) pickInProgressColocatedTask() (*ImportFileTask, error) {
	if len(c.inProgressColocatedTasks) > 0 {
		_, pickedTask := c.pickRandomFromListOfTasks(c.inProgressColocatedTasks)
		log.Debugf("picking in-progress colocated task: %v", pickedTask)
		return pickedTask, nil
	}
	return nil, fmt.Errorf("no in-progress colocated tasks to pick from")
}

func (c *ColocatedCappedRandomTaskPicker) pickShardedTask() (*ImportFileTask, error) {
	if !c.HasMoreShardedTasks() {
		return nil, fmt.Errorf("no more sharded tasks")
	}

	// try to pick a sharded pending task.
	task, err := c.pickPendingShardedTaskAsPerMaxTasks()
	if err != nil {
		return nil, fmt.Errorf("picking pending sharded task: %w", err)
	}
	if task != nil {
		return task, nil
	}

	// try to pick a sharded in-progress task.
	return c.pickInProgressShardedTask()
}

func (c *ColocatedCappedRandomTaskPicker) pickPendingShardedTaskAsPerMaxTasks() (*ImportFileTask, error) {
	if len(c.inProgressShardedTasks) < c.maxShardedTasksInProgress {
		if len(c.pendingShardedTasks) > 0 {
			taskIndex, pickedTask := c.pickRandomFromListOfTasks(c.pendingShardedTasks)
			c.pendingShardedTasks = append(c.pendingShardedTasks[:taskIndex], c.pendingShardedTasks[taskIndex+1:]...)
			c.inProgressShardedTasks = append(c.inProgressShardedTasks, pickedTask)
			log.Debugf("picking pending sharded task: %v", pickedTask)
			return pickedTask, nil
		}
	}
	log.Debugf("could not pick pending sharded task. inProgressShardedTasks: %v, maxShardedTasksInProgress: %v, pendingShardedTasks: %v",
		c.inProgressShardedTasks, c.maxShardedTasksInProgress, c.pendingShardedTasks)
	return nil, nil
}

func (c *ColocatedCappedRandomTaskPicker) pickInProgressShardedTask() (*ImportFileTask, error) {
	if len(c.inProgressShardedTasks) > 0 {
		_, pickedTask := c.pickRandomFromListOfTasks(c.inProgressShardedTasks)
		log.Debugf("picking in-progress sharded task: %v", pickedTask)
		return pickedTask, nil
	}
	return nil, fmt.Errorf("no in-progress sharded tasks to pick from")
}

func (c *ColocatedCappedRandomTaskPicker) MarkTaskAsDone(task *ImportFileTask) error {

	for i, t := range c.inProgressColocatedTasks {
		if t.ID == task.ID {
			c.inProgressColocatedTasks = append(c.inProgressColocatedTasks[:i], c.inProgressColocatedTasks[i+1:]...)
			c.doneTasks = append(c.doneTasks, task)
			return nil
		}
	}
	for i, t := range c.inProgressShardedTasks {
		if t.ID == task.ID {
			c.inProgressShardedTasks = append(c.inProgressShardedTasks[:i], c.inProgressShardedTasks[i+1:]...)
			c.doneTasks = append(c.doneTasks, task)
			return nil
		}
	}
	return fmt.Errorf("task [%v] not found in inProgressTasks: %v", task, c.inProgressTasks())
}

func (c *ColocatedCappedRandomTaskPicker) WaitForTasksBatchesTobeImported() error {
	// if for all in-progress tasks, all batches are submitted, then sleep for a bit
	allTasksAllBatchesSubmitted := true

	for _, task := range c.inProgressTasks() {
		taskAllBatchesSubmitted, err := c.state.AllBatchesSubmittedForTask(task.ID)
		if err != nil {
			if errors.As(err, &ErrTaskNotFound{}) {
				log.Infof("task [%v] not found in state. Assuming all batches NOT submitted for task", task)
				allTasksAllBatchesSubmitted = false
				break
			}
			return fmt.Errorf("checking if all batches are submitted for task: %v: %w", task, err)
		}
		if !taskAllBatchesSubmitted {
			allTasksAllBatchesSubmitted = false
			break
		}
	}

	if allTasksAllBatchesSubmitted {
		log.Infof("All batches submitted for all in-progress tasks. Sleeping")
		time.Sleep(time.Millisecond * 100)
	}
	return nil
}
