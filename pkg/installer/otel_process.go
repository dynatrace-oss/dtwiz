package installer

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
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

// WaitResult is a non-blocking check of the process exit channel.
// The first received cmd.Wait result is cached and returned on later calls.
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

func (p *ManagedProcess) PrintSummaryLine() {
	listeningPort := detectListeningPort(p.PID)
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

func PrintProcessSummary(procs []*ManagedProcess, settleDuration time.Duration) (aliveNames []string, alivePIDs []int) {
	if len(procs) == 0 {
		return
	}
	time.Sleep(settleDuration)
	fmt.Println()
	for _, p := range procs {
		p.PrintSummaryLine()
		exited, _ := p.WaitResult()
		if !exited {
			aliveNames = append(aliveNames, p.Name)
			alivePIDs = append(alivePIDs, p.PID)
		}
	}
	return
}

func detectListeningPort(pid int) string {
	out, err := exec.Command("lsof", "-a", "-i", "TCP", "-sTCP:LISTEN", "-p", strconv.Itoa(pid), "-Fn", "-P").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		if idx := strings.LastIndex(line, ":"); idx >= 0 {
			port := line[idx+1:]
			if port != "4317" && port != "4318" {
				return port
			}
		}
	}
	return ""
}
