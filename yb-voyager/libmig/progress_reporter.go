package main

import log "github.com/sirupsen/logrus"

type ProgressReporter struct {
}

func NewProgressReporter() *ProgressReporter {
	return &ProgressReporter{}
}

func (p *ProgressReporter) ImportFileStarted(tableID *TableID, totalProgressAmount int64) {
	log.Infof("Import started for table %s, total progress: %v", tableID, totalProgressAmount)
}

func (p *ProgressReporter) AddProgressAmount(tableID *TableID, progressAmount int64) {
	log.Infof("Add %v progress to table %s", progressAmount, tableID)
}
