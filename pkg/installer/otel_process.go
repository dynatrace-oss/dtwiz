package installer

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/dynatrace-oss/dtwiz/pkg/logger"
)

const (
	portPollInterval   = 500 * time.Millisecond
	portPollTimeout    = 15 * time.Second
	processSettleDelay = 3 * time.Second
)

type ManagedProcess struct {
	Name           string
	PID            int
	LogName        string
	exitResultCh   chan error
	hasExited      bool
	cachedWaitErr  error
	resultConsumed bool
}

func StartManagedProcess(name, logName string, cmd *exec.Cmd, logFile *os.File) (*ManagedProcess, error) {
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, err
	}

	exitCh := make(chan error, 1)
	go func() {
		exitCh <- cmd.Wait()
		logFile.Close()
	}()

	return &ManagedProcess{
		Name:         name,
		PID:          cmd.Process.Pid,
		LogName:      logName,
		exitResultCh: exitCh,
	}, nil
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

	statusLine := fmt.Sprintf("  %s (PID %d)", p.Name, p.PID)
	if hasExited {
		if waitErr != nil {
			statusLine += fmt.Sprintf("  [crashed: %v — check log for details]", waitErr)
		} else {
			statusLine += "  [exited cleanly]"
		}
	} else if listeningPort != "" {
		statusLine += fmt.Sprintf(" → http://localhost:%s", listeningPort)
	} else {
		statusLine += "  [running, port not detected]"
	}
	statusLine += fmt.Sprintf("  [log: %s]", p.LogName)
	fmt.Println(statusLine)
}

// PrintSummaryLine performs a one-shot port detection. Prefer PrintProcessSummary
// for multiple processes — it polls in parallel with a retry window.
func (p *ManagedProcess) PrintSummaryLine() {
	p.printLine(detectProcessListeningPort(p.PID))
}

func PrintProcessSummary(procs []*ManagedProcess, settleDuration time.Duration) (aliveNames []string, alivePIDs []int) {
	if len(procs) == 0 {
		return
	}
	logger.Debug("waiting for processes to settle", "count", len(procs), "settle", settleDuration)
	time.Sleep(settleDuration)

	started := 0
	notStarted := 0
	for _, p := range procs {
		exited, _ := p.WaitResult()
		if exited {
			notStarted++
		} else {
			started++
		}
	}
	logger.Debug("settle complete", "started", started, "not_started", notStarted)

	ports := make([]string, len(procs))
	fmt.Println()
	if started == 0 {
		logger.Debug("all processes exited during settle — skipping port detection")
	} else {
		if notStarted > 0 {
			fmt.Printf("  %d of %d service(s) started (%d failed) — looking up addresses...\n", started, len(procs), notStarted)
		} else {
			fmt.Printf("  %d service(s) started — looking up addresses...\n", started)
		}

		deadline := time.Now().Add(portPollTimeout)
		iteration := 0
		for time.Now().Before(deadline) {
			iteration++
			var mu sync.Mutex
			portsFound := 0
			remaining := 0
			var wg sync.WaitGroup
			for i, p := range procs {
				if ports[i] != "" {
					portsFound++
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
					port := detectProcessListeningPort(proc.PID)
					logger.Debug("port probe", "iteration", iteration, "pid", proc.PID, "name", proc.Name, "port", port)
					if port != "" {
						mu.Lock()
						ports[idx] = port
						portsFound++
						mu.Unlock()
					}
				}(i, p)
			}
			wg.Wait()
			logger.Debug("poll iteration complete", "iteration", iteration, "remaining", remaining, "ports_found", portsFound)
			if remaining == 0 || portsFound == started {
				logger.Debug("port detection done", "reason", map[bool]string{true: "all exited", false: "all ports found"}[remaining == 0])
				break
			}
			time.Sleep(portPollInterval)
		}
	}

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
