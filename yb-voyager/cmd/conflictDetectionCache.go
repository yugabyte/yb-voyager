package cmd

import (
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

type ConflictDetectionCache struct {
	sync.Mutex
	cache map[int64]*tgtdb.Event
}

func NewConflictDetectionCache() *ConflictDetectionCache {
	return &ConflictDetectionCache{
		cache: make(map[int64]*tgtdb.Event),
	}
}

func (c *ConflictDetectionCache) Put(event *tgtdb.Event) {
	c.Lock()
	defer c.Unlock()
	log.Infof("adding event(vsn=%d) %v to cache", event.Vsn, event.String())
	c.cache[event.Vsn] = event
}

func (c *ConflictDetectionCache) Remove(vsn int64) {
	c.Lock()
	defer c.Unlock()
	if _, ok := c.cache[vsn]; !ok {
		log.Infof("event %+v not found in cache, nothing to remove", vsn)
		return
	}
	log.Infof("removing event(vsn=%d) from cache", vsn)
	delete(c.cache, vsn)
}

func (c *ConflictDetectionCache) IsPresent(vsn int64) bool {
	c.Lock()
	defer c.Unlock()
	_, ok := c.cache[vsn]
	return ok
}

func (c *ConflictDetectionCache) WaitUntilNoConflicts(currentEvent *tgtdb.Event) {
	log.Infof("check if cache conflict with event(vsn=%d) %s", currentEvent.Vsn, currentEvent.String())
	// TODO: handle multiple schema properly
	currTable := currentEvent.TableName
	if currentEvent.SchemaName != "public" {
		currTable = fmt.Sprintf("%s.%s", currentEvent.SchemaName, currentEvent.TableName)
	}

	log.Infof("current table: %s", currTable)
	uniqueKeyColumns := TableToUniqueKeyColumns[currTable]

	// Make a copy of the cache to avoid parallel updates during iteration
	c.Lock()
	cacheCopy := make(map[int64]*tgtdb.Event)
	for k, v := range c.cache {
		cacheCopy[k] = v
	}
	c.Unlock()

	for _, event := range cacheCopy {
		table := event.TableName
		if event.SchemaName != "public" {
			table = fmt.Sprintf("%s.%s", event.SchemaName, event.TableName)
		}
		if table != currTable {
			continue
		}
		log.Infof("comparing event(vsn=%d) %s with event(vsn=%d) %s", currentEvent.Vsn, currentEvent.String(), event.Vsn, event.String())
		log.Infof("uniqueKeyColumns: %v", uniqueKeyColumns)
		// checking for the conflict
		for _, column := range uniqueKeyColumns {
			log.Infof("comparing column %s where event.Fields[column] = %s and currentEvent.Fields[column] = %s", column, *event.Fields[column], *currentEvent.Fields[column])
			// if the unique key column value is same, then we have a conflict
			if *event.Fields[column] == *currentEvent.Fields[column] {
				log.Infof("conflict detected for table %s, column %s, between value of cachedEvent(%s) and currentEvent(%s)", table, column, *event.Fields[column], *currentEvent.Fields[column])
				for c.IsPresent(event.Vsn) { // checking the main cache
					time.Sleep(5 * time.Second)
				}
				log.Infof("conflict resolved for table %s, column %s, between value of cachedEvent(%s) and currentEvent(%s)", table, column, *event.Fields[column], *currentEvent.Fields[column])
				// we can't return after just one conflict, because there can be multiple conflicts
				// for example, if we have 3 unique key columns conflicting with 3 different events
			}
		}
	}
}

func (c *ConflictDetectionCache) RemoveBatch(batch *tgtdb.EventBatch) {
	for _, event := range batch.Events {
		if c.IsPresent(event.Vsn) {
			time.Sleep(2 * time.Second)
			c.Remove(event.Vsn)
		}
	}
}
