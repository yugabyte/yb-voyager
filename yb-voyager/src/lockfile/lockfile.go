package lockfile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/nightlyone/lockfile"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Lockfile struct {
	fpath    string
	cmdName  string
	cmdPID   int
	lockfile lockfile.Lockfile
}

func NewLockfile(fpath string) *Lockfile {
	return &Lockfile{fpath: fpath, cmdPID: -1}
}

func (l *Lockfile) GetCmdName() string {
	if l.cmdName != "" {
		return l.cmdName
	}

	fname := filepath.Base(l.fpath)
	l.cmdName = fname[1 : len(fname)-len("Lockfile.lck")]
	l.cmdName = strings.Replace(l.cmdName, "-", " ", -1)
	return l.cmdName
}

func (l *Lockfile) GetCmdPID() (int, error) {
	if l.cmdPID != -1 {
		return l.cmdPID, nil
	}

	bytes, err := os.ReadFile(l.fpath)
	if err != nil {
		return -1, fmt.Errorf("failed to read lockfile %q: %w", l.fpath, err)
	}
	l.cmdPID, err = strconv.Atoi(strings.Trim(string(bytes), " \n"))
	if err != nil {
		return -1, fmt.Errorf("failed to parse PID from lockfile %q: %w", l.fpath, err)
	}
	return l.cmdPID, nil
}

func (l *Lockfile) IsPIDActive() bool {
	pid, err := l.GetCmdPID()
	if err != nil {
		return false
	}

	proc, _ := os.FindProcess(pid) // Always succeeds on Unix systems

	// here process.Signal(syscall.Signal(0)) will return error only if process is not running
	err = proc.Signal(syscall.Signal(0))
	if err != nil {
		log.Infof("process %d is not active", pid)
		return false
	}
	log.Infof("process %d is active", pid)
	return true
}

func (l *Lockfile) Lock() {
	var err error
	l.lockfile, err = lockfile.New(l.fpath)
	if err != nil {
		utils.ErrExit("Failed to create lockfile %q: %v\n", l.fpath, err)
	}

	err = l.lockfile.TryLock()
	if err == nil {
		return
	} else if err == lockfile.ErrBusy {
		utils.ErrExit("Another instance of yb-voyager '%s' is running for this migration", l.GetCmdName())
	} else {
		utils.ErrExit("Unable to lock the export-dir: %v\n", err)
	}
}

func (l *Lockfile) Unlock() {
	err := l.lockfile.Unlock()
	if err != nil {
		utils.ErrExit("Unable to unlock %q: %v\n", l.lockfile, err)
	}
}
