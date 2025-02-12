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
	"math"
	"os"
	"slices"
	"testing"

	"github.com/mroth/weightedrand/v2"
	"github.com/samber/lo"
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
		task, err := picker.Pick()
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
		task, err := picker.Pick()
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
		task, err := picker.Pick()
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

func TestColocatedAwareRandomTaskPickerMultipleTasksPerTableAdheresToMaxTasksInProgress(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	// multiple tasks for same table.
	_, shardedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.sharded", 1)
	testutils.FatalIfError(t, err)
	_, shardedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.sharded", 2)
	testutils.FatalIfError(t, err)
	_, colocatedTask1, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated", 3)
	testutils.FatalIfError(t, err)
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file4", ldataDir, "public.colocated", 3)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		shardedTask1,
		shardedTask2,
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
			shardedTask2.TableNameTup,
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
		if task != pickedTask2 && task != pickedTask1 {
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

	// mark task2, task3 as done
	err = picker.MarkTaskAsDone(pickedTask2)
	assert.NoError(t, err)
	err = picker.MarkTaskAsDone(pickedTask3)
	assert.NoError(t, err)

	// now, next task should be pickedTask4, which is not one of the previous tasks
	pickedTask4, err := picker.NextTask()
	assert.NoError(t, err)
	assert.NotContains(t, []*ImportFileTask{pickedTask1, pickedTask2, pickedTask3}, pickedTask4)
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == pickedTask4, "task: %v, pickedTask4: %v", task, pickedTask2)
	}

	// mark task4 as done
	err = picker.MarkTaskAsDone(pickedTask4)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerSingleTask(t *testing.T) {
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

	tasks := []*ImportFileTask{
		shardedTask1,
	}
	dummyYb := &dummyYb{
		shardedTables: []sqlname.NameTuple{
			shardedTask1.TableNameTup,
		},
	}

	picker, err := NewColocatedAwareRandomTaskPicker(2, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	pickedTask1, err := picker.NextTask()
	assert.NoError(t, err)

	// no matter how many times we call NextTask therefater,
	// it should return either pickedTask1
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == pickedTask1, "task: %v, pickedTask1: %v", task, pickedTask1)
	}

	// mark task1 as done
	err = picker.MarkTaskAsDone(pickedTask1)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerTasksEqualToMaxTasksInProgress(t *testing.T) {
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

	// 3 tasks, 3 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(3, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == shardedTask1 || task == colocatedTask1 || task == colocatedTask2, "task: %v, expected tasks = %v", task, tasks)
	}

	// mark task1 as done
	err = picker.MarkTaskAsDone(shardedTask1)
	assert.NoError(t, err)

	// now, next task should be either colocatedTask1 or colocatedTask2
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == colocatedTask1 || task == colocatedTask2, "task: %v, colocatedTask1: %v, colocatedTask2: %v", task, colocatedTask1, colocatedTask2)
	}

	// mark colocatedTask1, colocatedTask2 as done
	err = picker.MarkTaskAsDone(colocatedTask1)
	assert.NoError(t, err)
	err = picker.MarkTaskAsDone(colocatedTask2)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerMultipleTasksPerTableTasksEqualToMaxTasksInProgress(t *testing.T) {
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
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated1", 3)
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

	// 3 tasks, 3 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(3, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == shardedTask1 || task == colocatedTask1 || task == colocatedTask2, "task: %v, expected tasks = %v", task, tasks)
	}

	// mark task1 as done
	err = picker.MarkTaskAsDone(shardedTask1)
	assert.NoError(t, err)

	// now, next task should be either colocatedTask1 or colocatedTask2
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == colocatedTask1 || task == colocatedTask2, "task: %v, colocatedTask1: %v, colocatedTask2: %v", task, colocatedTask1, colocatedTask2)
	}

	// mark colocatedTask1, colocatedTask2 as done
	err = picker.MarkTaskAsDone(colocatedTask1)
	assert.NoError(t, err)
	err = picker.MarkTaskAsDone(colocatedTask2)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerTasksLessThanMaxTasksInProgress(t *testing.T) {
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

	// 3 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == shardedTask1 || task == colocatedTask1 || task == colocatedTask2, "task: %v, expected tasks = %v", task, tasks)
	}

	// mark task1 as done
	err = picker.MarkTaskAsDone(shardedTask1)
	assert.NoError(t, err)

	// now, next task should be either colocatedTask1 or colocatedTask2
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == colocatedTask1 || task == colocatedTask2, "task: %v, colocatedTask1: %v, colocatedTask2: %v", task, colocatedTask1, colocatedTask2)
	}

	// mark colocatedTask1, colocatedTask2 as done
	err = picker.MarkTaskAsDone(colocatedTask1)
	assert.NoError(t, err)
	err = picker.MarkTaskAsDone(colocatedTask2)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerMultipleTasksPerTableTasksLessThanMaxTasksInProgress(t *testing.T) {
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
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated1", 3)
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

	// 3 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == shardedTask1 || task == colocatedTask1 || task == colocatedTask2, "task: %v, expected tasks = %v", task, tasks)
	}

	// mark task1 as done
	err = picker.MarkTaskAsDone(shardedTask1)
	assert.NoError(t, err)

	// now, next task should be either colocatedTask1 or colocatedTask2
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == colocatedTask1 || task == colocatedTask2, "task: %v, colocatedTask1: %v, colocatedTask2: %v", task, colocatedTask1, colocatedTask2)
	}

	// mark colocatedTask1, colocatedTask2 as done
	err = picker.MarkTaskAsDone(colocatedTask1)
	assert.NoError(t, err)
	err = picker.MarkTaskAsDone(colocatedTask2)
	assert.NoError(t, err)

	// now, there should be no more tasks
	assert.False(t, picker.HasMoreTasks())
	_, err = picker.NextTask()
	assert.Error(t, err)
}

func TestColocatedAwareRandomTaskPickerAllShardedTasks(t *testing.T) {
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
	_, shardedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.sharded2", 2)
	testutils.FatalIfError(t, err)
	_, shardedTask3, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.sharded3", 3)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		shardedTask1,
		shardedTask2,
		shardedTask3,
	}
	dummyYb := &dummyYb{
		shardedTables: []sqlname.NameTuple{
			shardedTask1.TableNameTup,
			shardedTask2.TableNameTup,
			shardedTask3.TableNameTup,
		},
	}

	// 3 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == shardedTask1 || task == shardedTask2 || task == shardedTask3, "task: %v, expected tasks = %v", task, tasks)
	}
}

func TestColocatedAwareRandomTaskPickerAllShardedTasksChooser(t *testing.T) {
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
	_, shardedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.sharded2", 2)
	testutils.FatalIfError(t, err)
	_, shardedTask3, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.sharded3", 3)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		shardedTask1,
		shardedTask2,
		shardedTask3,
	}
	dummyYb := &dummyYb{
		shardedTables: []sqlname.NameTuple{
			shardedTask1.TableNameTup,
			shardedTask2.TableNameTup,
			shardedTask3.TableNameTup,
		},
	}

	// 3 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	picker.initializeChooser()
	// all sharded tables should have equal probability of being picked
	assertTableChooserPicksShardedAndColocatedAsExpected(t, picker.tableChooser, dummyYb.colocatedTables, dummyYb.shardedTables)

	// now pick one task. After picking that task, the chooser should have updated probabilities
	task, err := picker.NextTask()
	assert.NoError(t, err)

	updatedPendingColocatedTables := lo.Filter(dummyYb.colocatedTables, func(t sqlname.NameTuple, _ int) bool {
		return t != task.TableNameTup
	})
	updatedPendingShardedTables := lo.Filter(dummyYb.shardedTables, func(t sqlname.NameTuple, _ int) bool {
		return t != task.TableNameTup
	})

	assertTableChooserPicksShardedAndColocatedAsExpected(t, picker.tableChooser, updatedPendingColocatedTables, updatedPendingShardedTables)

}

func assertTableChooserPicksShardedAndColocatedAsExpected(t *testing.T, chooser *weightedrand.Chooser[sqlname.NameTuple, int],
	expectedColocatedTableNames []sqlname.NameTuple, expectedShardedTableNames []sqlname.NameTuple) {
	tableNameTuples := append(expectedColocatedTableNames, expectedShardedTableNames...)
	colocatedPickCounter := map[sqlname.NameTuple]int{}
	shardedPickCounter := map[sqlname.NameTuple]int{}

	for i := 0; i < 100000; i++ {
		tableName := chooser.Pick()
		assert.Contains(t, tableNameTuples, tableName)
		if slices.Contains(expectedColocatedTableNames, tableName) {
			colocatedPickCounter[tableName]++
		} else {
			shardedPickCounter[tableName]++
		}
	}
	fmt.Printf("colocatedPickCounter: %v\n", colocatedPickCounter)
	fmt.Printf("shardedPickCounter: %v\n", shardedPickCounter)
	totalColocatedTables := len(expectedColocatedTableNames)
	totalShardedTables := len(expectedShardedTableNames)

	assert.Equal(t, totalColocatedTables, len(colocatedPickCounter))
	assert.Equal(t, totalShardedTables, len(shardedPickCounter))

	// assert that all colocated tables have been picked almost equal number of times
	totalColocatedPicks := 0
	for _, v := range colocatedPickCounter {
		totalColocatedPicks += v
	}
	if totalColocatedPicks > 0 {
		expectedCountForEachColocatedTable := totalColocatedPicks / totalColocatedTables
		for _, v := range colocatedPickCounter {
			diff := math.Abs(float64(v - expectedCountForEachColocatedTable))
			diffPct := float64(diff) / float64(expectedCountForEachColocatedTable) * 100
			// pct difference from expected count should be less than 5%
			assert.Truef(t, diffPct < 5, "diff: %v, diffPct: %v", diff, diffPct)
		}
	}

	// assert that all sharded tables have been picked almost equal number of times
	totalShardedPicks := 0
	for _, v := range shardedPickCounter {
		totalShardedPicks += v
	}
	if totalShardedPicks > 0 {
		expectedCountForEachShardedTable := totalShardedPicks / totalShardedTables
		for _, v := range shardedPickCounter {
			diff := math.Abs(float64(v - expectedCountForEachShardedTable))
			diffPct := float64(diff) / float64(expectedCountForEachShardedTable) * 100
			// pct difference from expected count should be less than 5%
			assert.Truef(t, diffPct < 5, "diff: %v, diffPct: %v", diff, diffPct)
		}
	}

	// assert that sum of probability of picking any of the colocated tables should be same as probability of picking any of the sharded tables
	if totalColocatedPicks > 0 && totalShardedPicks > 0 {
		for _, v := range shardedPickCounter {
			diff := math.Abs(float64(v - totalColocatedPicks))
			diffPct := float64(diff) / float64(totalColocatedPicks) * 100
			// pct difference from expected count should be less than 5%
			assert.Truef(t, diffPct < 5, "diff: %v, diffPct: %v", diff, diffPct)
		}
	}
}

func TestColocatedAwareRandomTaskPickerAllColocatedTasks(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, colocatedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.colocated1", 1)
	testutils.FatalIfError(t, err)
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.colocated2", 2)
	testutils.FatalIfError(t, err)
	_, colocatedTask3, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated3", 3)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		colocatedTask1,
		colocatedTask2,
		colocatedTask3,
	}
	dummyYb := &dummyYb{
		colocatedTables: []sqlname.NameTuple{
			colocatedTask1.TableNameTup,
			colocatedTask2.TableNameTup,
			colocatedTask3.TableNameTup,
		},
	}

	// 3 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == colocatedTask1 || task == colocatedTask2 || task == colocatedTask3, "task: %v, expected tasks = %v", task, tasks)
	}
}

func TestColocatedAwareRandomTaskPickerAllColocatedTasksChooser(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, colocatedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.colocated1", 1)
	testutils.FatalIfError(t, err)
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.colocated2", 2)
	testutils.FatalIfError(t, err)
	_, colocatedTask3, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated3", 3)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		colocatedTask1,
		colocatedTask2,
		colocatedTask3,
	}
	dummyYb := &dummyYb{
		colocatedTables: []sqlname.NameTuple{
			colocatedTask1.TableNameTup,
			colocatedTask2.TableNameTup,
			colocatedTask3.TableNameTup,
		},
	}

	// 3 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// all colocated tables should have equal probability of being picked
	picker.initializeChooser()
	assertTableChooserPicksShardedAndColocatedAsExpected(t, picker.tableChooser, dummyYb.colocatedTables, dummyYb.shardedTables)

	// now pick one task. After picking that task, the chooser should have updated probabilities
	task, err := picker.NextTask()
	assert.NoError(t, err)

	updatedPendingColocatedTables := lo.Filter(dummyYb.colocatedTables, func(t sqlname.NameTuple, _ int) bool {
		return t != task.TableNameTup
	})
	updatedPendingShardedTables := lo.Filter(dummyYb.shardedTables, func(t sqlname.NameTuple, _ int) bool {
		return t != task.TableNameTup
	})

	assertTableChooserPicksShardedAndColocatedAsExpected(t, picker.tableChooser, updatedPendingColocatedTables, updatedPendingShardedTables)
}

func TestColocatedAwareRandomTaskPickerMixShardedColocatedTasks(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, colocatedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.colocated1", 1)
	testutils.FatalIfError(t, err)
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.colocated2", 2)
	testutils.FatalIfError(t, err)
	_, colocatedTask3, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated3", 3)
	testutils.FatalIfError(t, err)
	_, shardedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.sharded1", 1)
	testutils.FatalIfError(t, err)
	_, shardedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.sharded2", 2)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		colocatedTask1,
		colocatedTask2,
		colocatedTask3,
		shardedTask1,
		shardedTask2,
	}
	dummyYb := &dummyYb{
		colocatedTables: []sqlname.NameTuple{
			colocatedTask1.TableNameTup,
			colocatedTask2.TableNameTup,
			colocatedTask3.TableNameTup,
		},
		shardedTables: []sqlname.NameTuple{
			shardedTask1.TableNameTup,
			shardedTask2.TableNameTup,
		},
	}

	// 5 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// no matter how many times we call NextTask therefater,
	// it should return one of the tasks
	for i := 0; i < 100; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)
		assert.Truef(t, task == colocatedTask1 || task == colocatedTask2 || task == colocatedTask3 || task == shardedTask1 || task == shardedTask2, "task: %v, expected tasks = %v", task, tasks)
	}
}

func TestColocatedAwareRandomTaskPickerMixShardedColocatedTasksChooser(t *testing.T) {
	ldataDir, lexportDir, state, err := setupExportDirAndImportDependencies(2, 1024)
	testutils.FatalIfError(t, err)

	if ldataDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", ldataDir))
	}
	if lexportDir != "" {
		defer os.RemoveAll(fmt.Sprintf("%s/", lexportDir))
	}

	_, colocatedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.colocated1", 1)
	testutils.FatalIfError(t, err)
	_, colocatedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.colocated2", 2)
	testutils.FatalIfError(t, err)
	_, colocatedTask3, err := createFileAndTask(lexportDir, "file3", ldataDir, "public.colocated3", 3)
	testutils.FatalIfError(t, err)
	_, shardedTask1, err := createFileAndTask(lexportDir, "file1", ldataDir, "public.sharded1", 1)
	testutils.FatalIfError(t, err)
	_, shardedTask2, err := createFileAndTask(lexportDir, "file2", ldataDir, "public.sharded2", 2)
	testutils.FatalIfError(t, err)

	tasks := []*ImportFileTask{
		colocatedTask1,
		colocatedTask2,
		colocatedTask3,
		shardedTask1,
		shardedTask2,
	}
	dummyYb := &dummyYb{
		colocatedTables: []sqlname.NameTuple{
			colocatedTask1.TableNameTup,
			colocatedTask2.TableNameTup,
			colocatedTask3.TableNameTup,
		},
		shardedTables: []sqlname.NameTuple{
			shardedTask1.TableNameTup,
			shardedTask2.TableNameTup,
		},
	}

	// 5 tasks, 10 max tasks in progress
	picker, err := NewColocatedAwareRandomTaskPicker(10, tasks, state, dummyYb)
	testutils.FatalIfError(t, err)
	assert.True(t, picker.HasMoreTasks())

	// all colocated tables should have same probability of being picked
	// all sharded tables should have same probability of being picked
	// sum of probability of picking any of the colocated tables should be same as probability of picking any of the sharded tables
	picker.initializeChooser()
	assertTableChooserPicksShardedAndColocatedAsExpected(t, picker.tableChooser, dummyYb.colocatedTables, dummyYb.shardedTables)

	updatedPendingColocatedTables := dummyYb.colocatedTables
	updatedPendingShardedTables := dummyYb.shardedTables

	// now pick tasks one by one. After picking each task, the chooser should have updated probabilities
	for i := 0; i < 4; i++ {
		task, err := picker.NextTask()
		assert.NoError(t, err)

		updatedPendingColocatedTables = lo.Filter(updatedPendingColocatedTables, func(t sqlname.NameTuple, _ int) bool {
			return t != task.TableNameTup
		})
		updatedPendingShardedTables = lo.Filter(updatedPendingShardedTables, func(t sqlname.NameTuple, _ int) bool {
			return t != task.TableNameTup
		})

		assertTableChooserPicksShardedAndColocatedAsExpected(t, picker.tableChooser, updatedPendingColocatedTables, updatedPendingShardedTables)
	}
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

all of the above cases for
multipleTasksPerTable (importDataFileCase)
*/
