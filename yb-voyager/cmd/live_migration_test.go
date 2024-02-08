package cmd

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

func TestInsertAndUpdateGoToSameChannel(t *testing.T) {
	var err error
	idVal := "1"
	valVal := "abc"
	valValUpdated := "def"

	pk := map[string]*string{}
	pk["id"] = &idVal

	fields := map[string]*string{}
	fields["id"] = &idVal
	fields["val"] = &valValUpdated

	beforeFields := map[string]*string{}
	beforeFields["id"] = &idVal
	beforeFields["val"] = &valVal

	i := tgtdb.Event{Vsn: 1,
		Op:           "i",
		SchemaName:   "public",
		TableName:    "foo",
		Key:          pk,
		Fields:       beforeFields,
		ExporterRole: "source_db_exporter",
	}

	u := tgtdb.Event{Vsn: 1,
		Op:           "u",
		SchemaName:   "public",
		TableName:    "foo",
		Key:          pk,
		Fields:       fields,
		BeforeFields: beforeFields,
		ExporterRole: "source_db_exporter",
	}
	var evChans []chan *tgtdb.Event
	for i := 0; i < NUM_EVENT_CHANNELS; i++ {
		evChans = append(evChans, make(chan *tgtdb.Event, EVENT_CHANNEL_SIZE))
	}
	conflictDetectionCache = NewConflictDetectionCache(map[string][]string{}, evChans)
	valueConverter = &FormatterOpValueConverter{}
	tconf.TargetDBType = YUGABYTEDB
	hi := hashEvent(&i)
	hu := hashEvent(&u)
	assert.Equal(t, hi, hu, "hash of insert and update event are not the same")

	err = handleEvent(&i, evChans)
	if err != nil {
		t.Error(err)
	}
	err = handleEvent(&u, evChans)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, 2, len(evChans[hi]), "insert and update event are not in the same channel")

}

type FormatterOpValueConverter struct{}

func (fc *FormatterOpValueConverter) ConvertRow(tableName string, columnNames []string, row string) (string, error) {
	return row, nil
}

func (fc *FormatterOpValueConverter) ConvertEvent(ev *tgtdb.Event, table string, formatIfRequired bool) error {
	if formatIfRequired {
		for column, value := range ev.Key {
			formattedValue := fmt.Sprintf("'%s'", *value)
			ev.Key[column] = &formattedValue
		}
	}

	return nil
}

func (fc *FormatterOpValueConverter) GetTableNameToSchema() map[string]map[string]map[string]string {
	return nil
}
