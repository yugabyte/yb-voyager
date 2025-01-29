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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

type dummyYb struct {
	colocatedTables []sqlname.NameTuple
	shardedTables   []sqlname.NameTuple
}

func (d *dummyYb) IsTableColocated(tableName sqlname.NameTuple) (bool, error) {
	for _, t := range d.colocatedTables {
		if t.Key() == tableName.Key() {
			return true, nil
		}
	}
	return false, nil
}

func (d *dummyYb) IsDBColocated() (bool, error) {
	return len(d.colocatedTables) > 0, nil
}

func TestSequentialTaskPickerBasic(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, task1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.table1", 1)
	testutils.FatalIfError(t, err)
	_, task2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.table2", 2)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		task1,
		task2,
	}

	picker, err := NewSequentialTaskPicker(tasks, state)
	testutils.FatalIfError(t, err)

	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask, it should return the same task (first task)
	for i := 0; i < 10; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Equal(t, task1, task)
	}
}

func TestSequentialTaskPickerMarkTaskDone(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, task1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.table1", 1)
	testutils.FatalIfError(t, err)
	_, task2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.table2", 2)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		task1,
		task2,
	}

	picker, err := NewSequentialTaskPicker(tasks, state)
	testutils.FatalIfError(t, err)

	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask, it should return the same task (first task)
	for i := 0; i < 10; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Equal(t, task1, task)
	}

	// mark the second task as done; should return err
	err = picker.MarkTaskAsDone(task2)
	assert.Error(t, err)

	// mark the first task as done, now, the picker should return task2
	err = picker.MarkTaskAsDone(task1)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Equal(t, task2, task)
	}

	// mark the second task as done, then the picker should not have any tasks anymore
	err = picker.MarkTaskAsDone(task2)
	assert.NoError(t, err)
	assert.False(t, picker.HasMoreTasks())

	// marking any task as done now should return an error
	err = picker.MarkTaskAsDone(task1)
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerAdheresToMaxTasksInProgress(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, shardedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.sharded1", 1)
	testutils.FatalIfError(t, err)
	_, colocatedTask1, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.colocated1", 2)
	testutils.FatalIfError(t, err)
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated2", 3)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		shardedTask1,
		colocatedTask1,
		colocatedTask2,
	}
	dummyYb := &dummyYb{
		colocatedTables: []sqlname.NameTuple{
			colocatedTask1.TableNameTup,
			colocatedTask2.TableNameTup,
		},
		shardedTables: []sqlname.NameTuple{
			shardedTask1.TableNameTup,
		},
	}

	picker, err := NewColocatedAwareRandomTaskPicker(2, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// initially because maxInprogressTasks is 2, we will get 2 different tasks
	pickedTask1, err := picker.NextTask()
	assert.NoError(t, err)
	pickedTask2, err := picker.NextTask()
	assert.NoError(t, err)
	assert.NotEqual(t, pickedTask1, pickedTask2)

	// no matter how many times we call NextTask therefater,
	// it should return either pickedTask1 or pickedTask2
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == pickedTask1 || task == pickedTask2, "task: %v, pickedTask1: %v, pickedTask2: %v", task, pickedTask1, pickedTask2)
	}

	// mark task1 as done
	err = picker.MarkTaskAsDone(pickedTask1)
	assert.NoError(t, err)

	// keep picking tasks until we get a task that is not pickedTask2
	var pickedTask3 *ImportFileTask
	for {
		task, err := picker.NextTask()
		if err != nil {
			break
		}
		if task != pickedTask2 {
			pickedTask3 = task
			break
		}
	}

	// now, next task should be either pickedTask2 or pickedTask3
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == pickedTask2 || task == pickedTask3, "task: %v, pickedTask2: %v, pickedTask3: %v", task, pickedTask2, pickedTask3)
	}

	// mark task3 as done
	err = picker.MarkTaskAsDone(pickedTask3)
	assert.NoError(t, err)

	// now, next task should be pickedTask2
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == pickedTask2, "task: %v, pickedTask2: %v", task, pickedTask2)
	}

	// mark task2 as done
	err = picker.MarkTaskAsDone(pickedTask2)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

/*
singleTask
maxTasksInProgressEqualToTotalTasks
maxTasksInProgressGreaterThanTotalTasks
MixOfColocatedAndShardedTables - ensure all are getting picked with proper weights
	test that weights change when tasks are marked as done
AllColocatedTables - ensure all are getting picked with proper weights
	test that weights change when tasks are marked as done
AllShardedTables - ensure all are getting picked with proper weights
	test that weights change when tasks are marked as done
dynamicTasks - create 100s of tasks randmoly colocated/shared,
	get next tasks in a large loop,
	mark them as done occasionally,
	ensure all are getting picked at least once.
	ensure tasks getting picked remain constant between marking them as done. and adhere to max.

all of the above cases for
multipleTasksPerTable (importDataFileCase)
*/
