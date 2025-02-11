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
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

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

func TestSequentialTaskPickerResumePicksInProgressTask(t *testing.T) {
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

	// update the state of the first task to in progress
	fbp, err := NewFileBatchProducer(task1, state)
	testutils.FatalIfError(t, err)
	batch, err := fbp.NextBatch()
	assert.NoError(t, err)
	err = batch.MarkInProgress()
	assert.NoError(t, err)
	taskState, err := state.GetFileImportState(task1.FilePath, task1.TableNameTup)
	assert.NoError(t, err)
	assert.Equal(t, FILE_IMPORT_IN_PROGRESS, taskState)

	// simulate restart by creating a new picker
	slices.Reverse(tasks) // reorder the tasks so that the in progress task is at the end
	picker, err = NewSequentialTaskPicker(tasks, state)

	// no matter how many times we call NextTask, it should return the same task (first task)
	for i := 0; i < 10; i++ {
		task, err := picker.Pick()
		assert.NoError(t, err)
		assert.Equal(t, task1, task)
	}

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
