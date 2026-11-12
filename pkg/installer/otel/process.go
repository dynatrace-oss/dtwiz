package otel

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/display"
	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	portPollInterval   = 500 * time.Millisecond
	portPollTimeout    = 10 * time.Second
	processSettleDelay = 3 * time.Second
)

type ManagedProcess struct {
	Name            string
	PID             int
	LogName         string
	Entrypoint      string // script/entrypoint that was launched, used for process re-discovery on Windows
	IsExeclLauncher bool   // true when the process is expected to exec-spawn a child and exit (Python on Windows)
	portDetector    func(pid int) string
	exitResultCh    chan error
	hasExited       bool
	cachedWaitErr   error
	resultConsumed  bool
}

func (p *ManagedProcess) detectPort() string {
	if p.portDetector != nil {
		return p.portDetector(p.PID)
	}
	return detectProcessListeningPort(p.PID)
}

func StartManagedProcess(name, logName, entrypoint string, cmd *exec.Cmd, logFile *os.File) (*ManagedProcess, error) {
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}

	pid := cmd.Process.Pid
	logger.Debug("managed process started", "name", name, "pid", pid, "cmd", cmd.Path)

	exitCh := make(chan error, 1)
	mp := &ManagedProcess{
		Name:            name,
		PID:             pid,
		LogName:         logName,
		Entrypoint:      entrypoint,
		IsExeclLauncher: entrypoint != "",
		exitResultCh:    exitCh,
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Debug("managed process exited with error", "name", name, "pid", pid, "err", err)
		} else {
			logger.Debug("managed process exited cleanly", "name", name, "pid", pid)
		}
		exitCh <- err
		logFile.Close()
	}()

	return mp, nil
}

func (p *ManagedProcess) WaitResult() (exited bool, err error) {
	if p.resultConsumed {
		return p.hasExited, p.cachedWaitErr
	}
	select {
	case waitErr := <-p.exitResultCh:
		p.resultConsumed = true
		p.hasExited = true
		p.cachedWaitErr = waitErr
		return true, waitErr
	default:
		return false, nil
	}
}

func (p *ManagedProcess) printLine(listeningPort string) {
	hasExited, waitErr := p.WaitResult()

	logInfo := "log: " + p.LogName
	switch {
	case hasExited && waitErr != nil:
		display.StepFail(p.Name, fmt.Sprintf("crashed: %v · PID %d · %s", waitErr, p.PID, logInfo))
	case hasExited:
		display.StepWarn(p.Name, fmt.Sprintf("exited cleanly · PID %d · %s", p.PID, logInfo))
	case listeningPort != "":
		display.StepDone(p.Name, fmt.Sprintf("→ http://localhost:%s · PID %d · %s", listeningPort, p.PID, logInfo))
	default:
		display.StepDone(p.Name, fmt.Sprintf("running, port not detected · PID %d · %s", p.PID, logInfo))
	}
}

// PrintSummaryLine performs a one-shot port detection. Prefer PrintProcessSummary
// for multiple processes — it polls in parallel with a retry window.
func (p *ManagedProcess) PrintSummaryLine() {
	p.printLine(p.detectPort())
}

func PrintProcessSummary(procs []*ManagedProcess, settleDuration time.Duration) (aliveNames []string, alivePIDs []int) {
	if len(procs) == 0 {
		return
	}
	started := 0
	notStarted := 0
	ports := make([]string, len(procs))
	_ = display.RunStep("Started services", func(s *display.Step) error {
		s.Detail("waiting for processes to settle")
		logger.Debug("waiting for processes to settle", "count", len(procs), "settle", settleDuration)
		time.Sleep(settleDuration)

		started = 0
		notStarted = 0
		for _, p := range procs {
			exited, waitErr := p.WaitResult()
			if exited {
				if waitErr != nil {
					logger.Debug("settle: process crashed", "name", p.Name, "pid", p.PID, "err", waitErr)
				} else {
					logger.Debug("settle: process exited cleanly", "name", p.Name, "pid", p.PID)
				}
				notStarted++
			} else {
				logger.Debug("settle: process still running", "name", p.Name, "pid", p.PID)
				started++
			}
		}
		logger.Debug("settle complete", "started", started, "not_started", notStarted)

		// adoptExeclChildren handles the Windows-specific case where
		// opentelemetry-instrument calls os.execl, which on Windows spawns a child
		// process and exits the launcher cleanly. See otel_process_windows.go.
		adoptExeclChildren(procs, &started, &notStarted)

		if started == 0 {
			logger.Debug("all processes exited during settle — skipping port detection")
		} else {
			s.Detail("looking up addresses…")

			deadline := time.Now().Add(portPollTimeout)
			for time.Now().Before(deadline) {
				var mu sync.Mutex
				remaining := 0
				var wg sync.WaitGroup
				for i, p := range procs {
					if ports[i] != "" {
						continue
					}
					exited, _ := p.WaitResult()
					if exited {
						continue
					}
					remaining++
					wg.Add(1)
					go func(idx int, proc *ManagedProcess) {
						defer wg.Done()
						port := proc.detectPort()
						logger.Debug("port probe", "pid", proc.PID, "name", proc.Name, "port", port)
						if port != "" {
							mu.Lock()
							ports[idx] = port
							mu.Unlock()
						}
					}(i, p)
				}
				wg.Wait()
				portsFound := 0
				for _, port := range ports {
					if port != "" {
						portsFound++
					}
				}
				logger.Debug("poll iteration complete", "remaining", remaining, "ports_found", portsFound)
				if remaining == 0 || portsFound == started {
					logger.Debug("port detection done", "reason", map[bool]string{true: "all exited", false: "all ports found"}[remaining == 0])
					break
				}
				time.Sleep(portPollInterval)
			}
		}

		if started == 0 {
			return fmt.Errorf("none of the %d service(s) is running", len(procs))
		}
		if notStarted > 0 {
			s.Detail("%d of %d running", started, len(procs))
		} else {
			s.Detail("%d running", started)
		}
		return nil
	})

	for i, p := range procs {
		p.printLine(ports[i])
	}

	for _, p := range procs {
		exited, _ := p.WaitResult()
		if !exited {
			aliveNames = append(aliveNames, p.Name)
			alivePIDs = append(alivePIDs, p.PID)
		}
	}
	return
}
