package importdata

import (
	"context"
	"fmt"
	"sync"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"
)

const (
	DEFAULT_BATCH_SIZE = 20_000
)

//============================================================================

type EnumOpState int

const (
	NOT_STARTED EnumOpState = 1
	IN_PROGRESS EnumOpState = 2
	DONE        EnumOpState = 3
)

var stateNames = []string{"NOT_STARTED", "IN_PROGRESS", "DONE"}

func (v EnumOpState) String() string {
	if v < NOT_STARTED || v > DONE {
		return "INVALID_STATE"
	}
	return stateNames[v]
}

//============================================================================

type ImportFileOp struct {
	sync.Mutex
	Sema *semaphore.Weighted
	wg   sync.WaitGroup

	migState         *MigrationState
	progressReporter *ProgressReporter
	tdb              *TargetDB
	batchGen         *BatchGenerator

	// Input.
	FileName    string
	TableID     *TableID
	Desc        *DataFileDescriptor
	CopyCommand string
	BatchSize   int

	// Output.
	Err error

	state                     EnumOpState
	dataFile                  DataFile
	lastBatchFromPrevRun      *Batch
	pendingBatchesFromPrevRun []*Batch
	failedBatches             []*Batch
	submittedBatchCount       int
}

func NewImportFileOp(
	migState *MigrationState, progressReporter *ProgressReporter, tdb *TargetDB,
	fileName string, tableID *TableID, desc *DataFileDescriptor, sema *semaphore.Weighted) *ImportFileOp {

	return &ImportFileOp{
		Sema:             sema,
		migState:         migState,
		progressReporter: progressReporter,
		tdb:              tdb,
		FileName:         fileName,
		TableID:          tableID,
		Desc:             desc,
		BatchSize:        DEFAULT_BATCH_SIZE,

		batchGen: NewBatchGenerator(fileName, tableID, desc),
	}
}

func (op *ImportFileOp) Init() error {
	// IMPORTANT: This method can be called twice if --start-clean is provided.
	log.Infof("Initialise import state for %s => %s.", op.FileName, op.TableID)
	err := op.migState.PrepareForImport(op.TableID)
	if err != nil {
		return fmt.Errorf("prepare for import: %w", err)
	}
	err = op.recoverStateFromPrevRun()
	if err != nil {
		return fmt.Errorf("recover state from previous run: %w", err)
	}
	err = op.openDataFile()
	if err != nil {
		return fmt.Errorf("open data file: %w", err)
	}
	err = op.setCopyCommand()
	if err != nil {
		return fmt.Errorf("set COPY command: %w", err)
	}
	return nil
}

func (op *ImportFileOp) State() EnumOpState {
	return op.state
}

func (op *ImportFileOp) Clean() error {
	err := op.migState.CleanState(op.TableID)
	if err != nil {
		return fmt.Errorf("clean local state: %s", err)
	}
	err = op.tdb.TruncateTable(context.Background(), op.TableID)
	if err != nil {
		return fmt.Errorf("truncate table: %w", err)
	}
	return op.Init()
}

func (op *ImportFileOp) Run(ctx context.Context) (err error) {
	log.Infof("Run ImportFileOp: %s => %s [cmd: %q]", op.FileName, op.TableID, op.CopyCommand)

	// op.lastBatchFromPrevRun will be nil for first time execution.
	err = op.batchGen.Init(op.dataFile, op.lastBatchFromPrevRun)
	if err != nil {
		return fmt.Errorf("initialise batch generation: %w", err)
	}

	op.notifyImportFileStarted()

	defer func() {
		log.Infof("[%s] Wait until all submitted batches are done before returning.", op.TableID)
		op.wg.Wait()
		log.Infof("[%s] Finished processing all batches.", op.TableID)
		op.progressReporter.TableImportDone(op.TableID) // So that the ProgressBar is removed.
		if op.Err == nil {
			op.state = DONE
		} else {
			err = fmt.Errorf("import table %s: %w", op.TableID, op.Err)
		}
	}()

	for _, batch := range op.pendingBatchesFromPrevRun {
		if op.Err != nil {
			break
		}
		op.submitBatch(batch)
	}

	for op.Err == nil {
		batch, eof, err := op.batchGen.NextBatch(op.BatchSize)
		if batch != nil {
			op.state = IN_PROGRESS
			err2 := op.migState.MarkBatchPending(batch)
			if err2 != nil {
				return err2
			}
			op.submitBatch(batch)
		}
		if eof {
			log.Infof("Done splitting file %s", op.FileName)
			return nil
		}
		if err != nil {
			return fmt.Errorf("prepare batch: %w", err)
		}
	}
	log.Errorf("[%s] Stopping batch generation. Batch %d failed with error: %s", op.TableID, op.failedBatches[0].BatchNumber, op.Err)
	return op.Err
}

func (op *ImportFileOp) recoverStateFromPrevRun() error {
	var err error
	log.Infof("Recover state from previous run: %q", op.TableID)

	op.lastBatchFromPrevRun, err = op.migState.GetLastBatch(op.TableID)
	if err != nil {
		return fmt.Errorf("find last batch: %w", err)
	}
	log.Infof("last batch from previous run:\n%s", spew.Sdump(op.lastBatchFromPrevRun))

	op.pendingBatchesFromPrevRun, err = op.migState.PendingBatches(op.TableID)
	if err != nil {
		return fmt.Errorf("find pending batches: %w", err)
	}
	slices.SortFunc(op.pendingBatchesFromPrevRun, func(b1, b2 *Batch) bool {
		// Keep the FAILED batches ahead of other pending batches.
		// Keep both groups ordered by BatchNumber.
		switch true {
		case b1.Err == "" && b2.Err == "":
			return b1.BatchNumber < b2.BatchNumber
		case b1.Err != "" && b2.Err != "":
			return b1.BatchNumber < b2.BatchNumber
		case b1.Err != "":
			return false // b1 has error. Keep it ahead of b2.
		case b2.Err != "":
			return true
		}
		panic("UNREACHABLE CASE")
	})
	// Set state.
	if op.lastBatchFromPrevRun == nil {
		op.state = NOT_STARTED
	} else if op.lastBatchFromPrevRun.IsFinalBatch && len(op.pendingBatchesFromPrevRun) == 0 {
		// Batch generation is done && all batches are imported ==> DONE .
		op.state = DONE
	} else {
		op.state = IN_PROGRESS
	}
	return nil
}

func (op *ImportFileOp) openDataFile() error {
	// Start from where we left off.
	offset := int64(0)
	if op.lastBatchFromPrevRun != nil {
		offset = op.lastBatchFromPrevRun.EndOffsetInBaseFile
	}

	// openDataFile() can be called twice in case of --start-clean.
	if op.dataFile != nil {
		op.dataFile.Close()
		op.dataFile = nil
	}
	log.Infof("Open DataFile %q and jump to offset %v.", op.FileName, offset)
	op.dataFile = NewDataFile(op.FileName, offset, op.Desc)
	err := op.dataFile.Open()
	if err != nil {
		return fmt.Errorf("open data file: %w", err)
	}
	return nil
}

func (op *ImportFileOp) setCopyCommand() error {
	log.Infof("Determine COPY command for the table %q.", op.TableID)
	if op.CopyCommand == "" {
		cmd, err := op.dataFile.GetCopyCommand(op.TableID)
		if err != nil {
			return fmt.Errorf("determine COPY command from file contents: %w", err)
		}
		op.CopyCommand = cmd
	}
	log.Infof("COPY command for %s => %s", op.TableID, op.CopyCommand)
	return nil
}

func (op *ImportFileOp) notifyImportFileStarted() {
	log.Infof("Notify import file start [%s]", op.TableID)

	op.progressReporter.ImportFileStarted(op.TableID, op.dataFile.Size())

	if op.lastBatchFromPrevRun != nil {
		p := op.lastBatchFromPrevRun.EndOffsetInBaseFile
		for _, batch := range op.pendingBatchesFromPrevRun {
			p -= batch.SizeInBaseFile()
		}
		op.progressReporter.AddProgressAmount(op.TableID, p)
	}
}

func (op *ImportFileOp) submitBatch(batch *Batch) {
	op.wg.Add(1)
	op.Sema.Acquire(context.Background(), 1)
	log.Infof("Submitting batch %s %d", op.TableID, batch.BatchNumber)
	go func() {
		err := op.importBatch(batch)
		if err != nil {
			log.Errorf("Failed to import batch %s %d: %s", op.TableID, batch.BatchNumber, err)
			op.Lock()
			if op.Err == nil {
				op.Err = err
			}
			op.Unlock()
		}
		op.Sema.Release(1)
		op.wg.Done()
	}()
	op.submittedBatchCount++
	if op.submittedBatchCount <= 3 {
		// Synchronously import first 3 batches.
		op.wg.Wait()
	}
}

func (op *ImportFileOp) importBatch(batch *Batch) error {
	log.Infof("Import batch %s %d: file %q, start: %v, end: %v",
		op.TableID, batch.BatchNumber, batch.FileName, batch.StartOffset, batch.EndOffset)
	ctx := context.Background()
	copyCommand := fmt.Sprintf(op.CopyCommand, batch.RecordCount)
	n, err := op.tdb.Copy(ctx, copyCommand, batch)
	if err != nil {
		log.Errorf("COPY batch %s %d failed: %s", batch.TableID, batch.BatchNumber, err)
		err2 := op.migState.MarkBatchFailed(batch, err)
		if err2 != nil {
			log.Errorf("Error while marking batch %s %d as failed: %s", batch.TableID, batch.BatchNumber, err2)
			err = fmt.Errorf("batch import failed with %q. couldn't mark batch as failed: %w", err, err2)
		}
		op.Lock()
		op.failedBatches = append(op.failedBatches, batch)
		op.Unlock()
		return fmt.Errorf("COPY: %w", err)
	}
	batch.NumRecordsImported = n
	err = op.migState.MarkBatchDone(batch)
	if err != nil {
		return fmt.Errorf("mark batch %s %d as DONE: %w", batch.TableID, batch.BatchNumber, err)
	}
	log.Infof("Imported %v/%v records from batch %s %d.", batch.NumRecordsImported, batch.RecordCount, batch.TableID, batch.BatchNumber)
	op.progressReporter.AddProgressAmount(op.TableID, batch.SizeInBaseFile())
	return nil
}

/*
func debugPrintBatch(batch *Batch) {
	r, err := batch.Reader()
	if err != nil {
		panic(err)
	}
	bs, err := io.ReadAll(r)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s", string(bs))
}

func debugPrintBatch2(batch *Batch) {
	r, err := batch.Reader()
	if err != nil {
		panic(err)
	}

	var bs []byte
	buf := make([]byte, 100)
	for {
		n, err := r.Read(buf)
		bs = append(bs, buf[:n]...)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
	}
	fmt.Printf("%s", string(bs))
}
*/
