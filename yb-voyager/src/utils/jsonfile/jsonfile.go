package jsonfile

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type JsonFile[T any] struct {
	sync.Mutex
	FilePath string
}

func NewJsonFile[T any](filePath string) *JsonFile[T] {
	return &JsonFile[T]{FilePath: filePath}
}

func (j *JsonFile[T]) Create(obj *T) error {
	j.Lock()
	defer j.Unlock()
	_, err := os.Create(j.FilePath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", j.FilePath, err)
	}
	return j.write(obj)
}

func (j *JsonFile[T]) Read() (*T, error) {
	j.Lock()
	defer j.Unlock()
	return j.read()
}

func (j *JsonFile[T]) read() (*T, error) {
	bs, err := os.ReadFile(j.FilePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", j.FilePath, err)
	}
	if len(bs) == 0 {
		return nil, fmt.Errorf("file %s is empty", j.FilePath)
	}
	obj := new(T)
	err = json.Unmarshal(bs, obj)
	if err != nil {
		return nil, fmt.Errorf("unmarshal json: %w", err)
	}
	return obj, nil
}

func (j *JsonFile[T]) Update(fn func(*T)) error {
	j.Lock()
	defer j.Unlock()
	var obj *T
	var err error
	if utils.FileOrFolderExists(j.FilePath) {
		obj, err = j.read()
		if err != nil {
			return err
		}
	} else {
		obj = new(T)
	}

	fn(obj)
	return j.write(obj)
}

func (j *JsonFile[T]) write(obj *T) error {
	bs, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	err = os.WriteFile(j.FilePath, bs, 0644)
	if err != nil {
		return fmt.Errorf("write file %s: %w", j.FilePath, err)
	}
	return nil
}

func (j *JsonFile[T]) Delete() error {
	j.Lock()
	defer j.Unlock()
	return os.Remove(j.FilePath)
}
