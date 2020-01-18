package autorestart

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
)

const defaultPeriod = time.Second * 1

// Exported variables
var (
	// WatchFilename is a filename to watch for, by default it is a program binary
	WatchFilename string
	// WatchPeriod is a period of time to check for changes, default is `1 * time.Second
	WatchPeriod = defaultPeriod
	// RestartFunc is a function called for restart,
	// default is `RestartByExec` you can use `SendSIGUSR2` or your custom function
	RestartFunc = RestartByExec
)

var (
	executableArgs []string
	executableEnvs []string
	executablePath string
	ticker         *time.Ticker
	startFileInfo  os.FileInfo
	listeners      []chan bool
)

func init() {
	listeners = make([]chan bool, 0)
	executableArgs = os.Args
	executableEnvs = os.Environ()
	executablePath, _ = filepath.Abs(os.Args[0])
	WatchFilename = executablePath
}

// StartWatcher starts timer
func StartWatcher() {
	ticker = time.NewTicker(WatchPeriod)
	go watcher()
}

// GetNotifier returns a channel, it will recived message before restart
// channel is synchronous and must be readed to continue
func GetNotifier() (c chan bool) {
	c = make(chan bool)
	listeners = append(listeners, c)
	return c
}

func watcher() {
	for range ticker.C {
		if isChanged() {
			notify()
			RestartFunc()
		}
	}
}

func isChanged() bool {
	return isChangedByStat()
}

func isChangedByStat() bool {
	fileinfo, err := os.Stat(WatchFilename)
	if err == nil {
		// first update
		if startFileInfo == nil {
			startFileInfo = fileinfo
			return false
		}

		if startFileInfo.ModTime() != fileinfo.ModTime() ||
			startFileInfo.Size() != fileinfo.Size() {
			return true
		}

		return false
	}

	log.Printf("cannot find %s: %s", WatchFilename, err)
	return false
}

func notify() {
	for _, c := range listeners {
		c <- true
	}
}

// RestartByExec calls `syscall.Exec()` to restart app
func RestartByExec() {
	binary, err := exec.LookPath(executablePath)
	if err != nil {
		log.Printf("Error: %s", err)
		return
	}
	time.Sleep(1 * time.Second)

	if runtime.GOOS == "windows" {
		err = RestartUnderWindows(binary)
		if err != nil {
			return
		}
		log.Println("exiting ...")
		os.Exit(1)
	}

	execErr := syscall.Exec(binary, executableArgs, executableEnvs)
	if execErr != nil {
		log.Printf("error: %s %v", binary, execErr)
	}
}

// RestartUnderWindows use cmd.exe for starting new binary and exit parent process
func RestartUnderWindows(binary string) error {

	s := []string{"/C", "start", binary}
	s = append(s, executableArgs...)

	cmd := exec.Command("cmd.exe", s...)
	err := cmd.Run()
	if err != nil {
		log.Printf("error: %s %v", binary, err)
		return err
	}

	return nil
}

//disabled, problems with building for windows
// SendSIGUSR2 SIGUSR2 is used in github.com/facebookgo/grace package
/*func SendSIGUSR2() {
	if proc, err := os.FindProcess(os.Getpid()); err != nil {
		log.Printf("FindProcess: %s", err)
		return
	} else {
		proc.Signal(syscall.SIGUSR2)
	}
}
*/
