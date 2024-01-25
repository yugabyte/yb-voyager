package jsonfile

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJsonFile(t *testing.T) {
	type Person struct {
		Name string
	}
	var person *Person
	jf := NewJsonFile[Person]("person.json")
	_ = jf.Delete()
	person, err := jf.Read()
	assert.ErrorIs(t, err, os.ErrNotExist)
	assert.Nil(t, person)
	err = jf.Update(func(p *Person) {
		p.Name = "John Doe"
	})
	assert.Nil(t, err)
	person, err = jf.Read()
	assert.Nil(t, err)
	assert.NotNil(t, person)
	assert.Equal(t, "John Doe", person.Name)
	err = jf.Update(func(p *Person) {
		p.Name = "John Smith"
	})
	assert.Nil(t, err)
	person, err = jf.Read()
	assert.Nil(t, err)
	assert.Equal(t, "John Smith", person.Name)
	err = jf.Delete()
	assert.Nil(t, err)
}
