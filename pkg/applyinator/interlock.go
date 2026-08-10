package applyinator

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const bootIDPath = "/proc/sys/kernel/random/boot_id"

// interlockOwner identifies the process that created an interlock file, so a
// later process can distinguish a live holder from one that was killed.
type interlockOwner struct {
	PID     int
	BootID  string
	Written time.Time
}

func newInterlockOwner(now time.Time) interlockOwner {
	return interlockOwner{PID: os.Getpid(), BootID: currentBootID(), Written: now}
}

func currentBootID() string {
	b, err := os.ReadFile(bootIDPath)
	if err != nil {
		return "" // non-Linux or restricted /proc: degrade to PID-only liveness
	}
	return strings.TrimSpace(string(b))
}

func (o interlockOwner) marshal() []byte {
	return []byte(fmt.Sprintf("pid=%d\nboot=%s\ntime=%s\n",
		o.PID, o.BootID, o.Written.Format(time.UnixDate)))
}

// parseInterlockOwner reports ok=false for any file it does not recognise —
// including the empty file written by `touch` in an older install.sh and the
// bare timestamp written by system-agent <= v0.3.16. Callers must keep the
// legacy path for those.
func parseInterlockOwner(contents []byte) (interlockOwner, bool) {
	var o interlockOwner
	var sawPID bool
	s := bufio.NewScanner(strings.NewReader(string(contents)))
	for s.Scan() {
		k, v, found := strings.Cut(strings.TrimSpace(s.Text()), "=")
		if !found {
			continue
		}
		switch k {
		case "pid":
			n, err := strconv.Atoi(v)
			if err != nil {
				return interlockOwner{}, false
			}
			o.PID, sawPID = n, true
		case "boot":
			o.BootID = v
		case "time":
			if t, err := time.Parse(time.UnixDate, v); err == nil {
				o.Written = t
			}
		}
	}
	return o, sawPID
}

// isAlive reports whether the writing process is still running. A boot ID that
// differs from the current one means the file predates a reboot, so the PID —
// which recycles across reboots — must not be trusted.
func (o interlockOwner) isAlive() bool {
	if cur := currentBootID(); cur != "" && o.BootID != "" && cur != o.BootID {
		return false
	}
	if o.PID <= 0 {
		return false
	}
	if o.PID == os.Getpid() {
		return true
	}
	p, err := os.FindProcess(o.PID)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil

}
