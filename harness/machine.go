package harness

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// Machine is the host fingerprint recorded with every result. Results carrying
// two different MachineID values are never merged into one series: a change of
// machine starts a new series rather than being rescaled onto the old one.
type Machine struct {
	// MachineID is a truncated SHA-256 of the host's machine identifier, not
	// the identifier itself. It is stable per host and comparable across
	// results, and publishing the raw value would put a host identifier in a
	// public repository for no measurement benefit.
	MachineID string `json:"machine_id"`
	// Runner names where the measurement happened, in the operator's words:
	// "github-hosted:ubuntu-latest", "local:<something>", and — if the
	// hardware question is ever answered yes — a dedicated label.
	Runner        string `json:"runner"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CPUModel      string `json:"cpu_model"`
	CPUCores      int    `json:"cpu_cores"`
	MemTotalBytes int64  `json:"mem_total_bytes"`
	Kernel        string `json:"kernel"`
	CPUGovernor   string `json:"cpu_governor,omitempty"`
	// Citable is false for every host this repository has not committed to
	// RUNNER.md as a measurement machine. It is false everywhere today.
	Citable bool `json:"citable"`
}

// HostState is the ambient condition around a single repetition. A repetition
// run under load is not thrown away; it is published with the load recorded,
// so a reader can see whether the spread has an explanation.
type HostState struct {
	LoadAvg1     float64 `json:"load_avg_1"`
	FreeMemBytes int64   `json:"free_mem_bytes"`
}

// Fingerprint reads what the host is willing to tell us. Every field is
// best-effort: a missing field is empty rather than invented, because the
// plan's host-sensor fields depend on a provider exposing them.
func Fingerprint(runner string) Machine {
	m := Machine{
		Runner:        runner,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		CPUCores:      runtime.NumCPU(),
		CPUModel:      cpuModel(),
		MemTotalBytes: memTotalBytes(),
		Kernel:        uname(),
		CPUGovernor:   cpuGovernor(),
		Citable:       false,
	}
	m.MachineID = hashID(rawMachineID())
	return m
}

func hashID(raw string) string {
	if raw == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])[:16]
}

func rawMachineID() string {
	for _, p := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		if b, err := os.ReadFile(p); err == nil { //nolint:gosec // fixed system paths.
			if s := strings.TrimSpace(string(b)); s != "" {
				return s
			}
		}
	}
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
		if err == nil {
			for line := range strings.SplitSeq(string(out), "\n") {
				if strings.Contains(line, "IOPlatformUUID") {
					if i := strings.LastIndex(line, "= "); i >= 0 {
						return strings.Trim(strings.TrimSpace(line[i+2:]), `"`)
					}
				}
			}
		}
	}
	host, _ := os.Hostname()
	return host
}

func cpuModel() string {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("/usr/sbin/sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck // read-only.
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") || strings.HasPrefix(line, "Model") {
			if i := strings.Index(line, ":"); i >= 0 {
				return strings.TrimSpace(line[i+1:])
			}
		}
	}
	return ""
}

func memTotalBytes() int64 {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("/usr/sbin/sysctl", "-n", "hw.memsize").Output(); err == nil {
			if v, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
				return v
			}
		}
		return 0
	}
	return procMeminfoKB("MemTotal:") * 1024
}

func procMeminfoKB(key string) int64 {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0
	}
	defer f.Close() //nolint:errcheck // read-only.
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), key) {
			fields := strings.Fields(sc.Text())
			if len(fields) >= 2 {
				if v, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
					return v
				}
			}
		}
	}
	return 0
}

func uname() string {
	out, err := exec.Command("uname", "-sr").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func cpuGovernor() string {
	b, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_governor")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// ReadHostState samples load and free memory around a repetition.
func ReadHostState() HostState {
	s := HostState{}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		if fields := strings.Fields(string(b)); len(fields) > 0 {
			s.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
		}
		s.FreeMemBytes = procMeminfoKB("MemAvailable:") * 1024
		return s
	}
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("/usr/sbin/sysctl", "-n", "vm.loadavg").Output(); err == nil {
			fields := strings.Fields(strings.Trim(strings.TrimSpace(string(out)), "{} "))
			if len(fields) > 0 {
				s.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
			}
		}
	}
	return s
}
