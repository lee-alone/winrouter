//go:build windows

package core

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
)

type TUNExperiment struct {
	PID                   int                 `json:"pid"`
	Started               bool                `json:"started"`
	Interface             *interfaces.Adapter `json:"interface,omitempty"`
	Stopped               bool                `json:"stopped"`
	StopMethod            string              `json:"stop_method,omitempty"`
	GracefulStop          bool                `json:"graceful_stop"`
	JobConstrained        bool                `json:"job_constrained"`
	NetworkChangeDetected bool                `json:"network_change_detected"`
	ChangedInterface      string              `json:"changed_interface,omitempty"`
	SleepResumeDetected   bool                `json:"sleep_resume_detected"`
	ResumeActionMillis    int64               `json:"resume_action_millis,omitempty"`
	InterfaceCleaned      bool                `json:"interface_cleaned"`
	RoutesCleaned         bool                `json:"routes_cleaned"`
	RemainingRoutes       []routes.Route      `json:"remaining_routes"`
	TCPProbes             []TCPProbe          `json:"tcp_probes,omitempty"`
	UDPProbes             []UDPProbe          `json:"udp_probes,omitempty"`
	ICMPProbes            []ICMPProbe         `json:"icmp_probes,omitempty"`
	DNSProbes             []DNSProbe          `json:"dns_probes,omitempty"`
	IPv6Probes            []IPv6Probe         `json:"ipv6_probes,omitempty"`
	CoreOutput            string              `json:"core_output,omitempty"`
	ElapsedMillis         int64               `json:"elapsed_millis"`
}

type TCPProbe struct {
	Target        string `json:"target"`
	Connected     bool   `json:"connected"`
	LocalAddress  string `json:"local_address,omitempty"`
	RemoteAddress string `json:"remote_address,omitempty"`
	Error         string `json:"error,omitempty"`
}

type UDPProbe struct {
	Target        string `json:"target"`
	Sent          bool   `json:"sent"`
	Responded     bool   `json:"responded"`
	LocalAddress  string `json:"local_address,omitempty"`
	RemoteAddress string `json:"remote_address,omitempty"`
	ResponseBytes int    `json:"response_bytes,omitempty"`
	Error         string `json:"error,omitempty"`
}

type ICMPProbe struct {
	Target  string `json:"target"`
	Reached bool   `json:"reached"`
	Output  string `json:"output,omitempty"`
	Error   string `json:"error,omitempty"`
}

type DNSProbe struct {
	Name      string   `json:"name"`
	Addresses []string `json:"addresses,omitempty"`
	Resolved  bool     `json:"resolved"`
	Error     string   `json:"error,omitempty"`
}

type IPv6Probe struct {
	DNSName           string   `json:"dns_name"`
	DNSBlocked        bool     `json:"dns_blocked"`
	DNSAddresses      []string `json:"dns_addresses,omitempty"`
	DNSError          string   `json:"dns_error,omitempty"`
	ConnectionTarget  string   `json:"connection_target"`
	ConnectionBlocked bool     `json:"connection_blocked"`
	ConnectionError   string   `json:"connection_error,omitempty"`
}

func RunTUNExperiment(ctx context.Context, executable string, config []byte, interfaceName, expectedPrefix string, hold time.Duration, tcpTargets, udpTargets, icmpTargets, dnsNames []string, ipv6Probe *IPv6Probe, watchedInterfaces []string) (report TUNExperiment, err error) {
	startedAt := time.Now()
	defer func() { report.ElapsedMillis = time.Since(startedAt).Milliseconds() }()

	if _, err := Validate(ctx, executable, LockedVersion, LockedSHA256, config); err != nil {
		return report, err
	}
	file, err := os.CreateTemp("", "winrouter-tun-experiment-*.json")
	if err != nil {
		return report, fmt.Errorf("create experiment config: %w", err)
	}
	configPath := file.Name()
	defer os.Remove(configPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return report, fmt.Errorf("restrict experiment config: %w", err)
	}
	if _, err := file.Write(config); err != nil {
		file.Close()
		return report, fmt.Errorf("write experiment config: %w", err)
	}
	if err := file.Close(); err != nil {
		return report, fmt.Errorf("close experiment config: %w", err)
	}

	var output bytes.Buffer
	job, err := createKillOnCloseJob()
	if err != nil {
		return report, err
	}
	defer windows.CloseHandle(job)
	command := exec.Command(executable, "run", "-c", configPath)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		return report, fmt.Errorf("start sing-box: %w", err)
	}
	report.PID = command.Process.Pid
	if err := assignProcessToJob(job, command.Process.Pid); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return report, err
	}
	report.JobConstrained = true
	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	defer func() {
		if command.Process != nil && !report.Stopped {
			_ = command.Process.Kill()
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
			}
		}
		report.CoreOutput = strings.TrimSpace(output.String())
		if err != nil && report.CoreOutput != "" {
			err = fmt.Errorf("%w; core output: %s", err, report.CoreOutput)
		}
	}()

	adapter, err := waitForTUN(ctx, exited, interfaceName, expectedPrefix, 15*time.Second)
	if err != nil {
		return report, fmt.Errorf("wait for TUN: %w", err)
	}
	report.Started = true
	report.Interface = &adapter
	select {
	case <-ctx.Done():
		return report, ctx.Err()
	case processErr := <-exited:
		return report, fmt.Errorf("sing-box exited before route stabilization: %w", processErr)
	case <-time.After(1500 * time.Millisecond):
	}
	var probeFailed bool
	for _, target := range tcpTargets {
		probe := TCPProbe{Target: target}
		connection, probeErr := net.DialTimeout("tcp", target, 5*time.Second)
		if probeErr != nil {
			probe.Error = probeErr.Error()
			probeFailed = true
		} else {
			probe.Connected = true
			probe.LocalAddress = connection.LocalAddr().String()
			probe.RemoteAddress = connection.RemoteAddr().String()
			_ = connection.Close()
		}
		report.TCPProbes = append(report.TCPProbes, probe)
	}
	for _, target := range udpTargets {
		probe := runNTPUDPProbe(target)
		report.UDPProbes = append(report.UDPProbes, probe)
		if !probe.Responded {
			probeFailed = true
		}
	}
	for _, target := range icmpTargets {
		probe := runICMPProbe(ctx, target)
		report.ICMPProbes = append(report.ICMPProbes, probe)
		if !probe.Reached {
			probeFailed = true
		}
	}
	for _, name := range dnsNames {
		probe := runDNSProbe(ctx, name)
		report.DNSProbes = append(report.DNSProbes, probe)
		if !probe.Resolved {
			probeFailed = true
		}
	}
	if ipv6Probe != nil {
		probe := runIPv6Probe(ctx, ipv6Probe.DNSName, ipv6Probe.ConnectionTarget)
		report.IPv6Probes = append(report.IPv6Probes, probe)
		if !probe.DNSBlocked || !probe.ConnectionBlocked {
			probeFailed = true
		}
	}

	holdTimer := time.NewTimer(hold)
	defer holdTimer.Stop()
	changeTicker := time.NewTicker(250 * time.Millisecond)
	defer changeTicker.Stop()
	lastTick := time.Now()
	var resumeDetectedAt time.Time
holdLoop:
	for {
		select {
		case <-ctx.Done():
			return report, ctx.Err()
		case processErr := <-exited:
			report.Stopped = true
			return report, fmt.Errorf("sing-box exited during hold period: %w", processErr)
		case <-holdTimer.C:
			break holdLoop
		case <-changeTicker.C:
			now := time.Now()
			if now.Sub(lastTick) > 5*time.Second {
				report.SleepResumeDetected = true
				resumeDetectedAt = now
				break holdLoop
			}
			lastTick = now
			if changed := unavailableInterface(watchedInterfaces); changed != "" {
				report.NetworkChangeDetected = true
				report.ChangedInterface = changed
				break holdLoop
			}
		}
	}

	stopErr := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(command.Process.Pid))
	if stopErr == nil {
		report.StopMethod = "ctrl_break"
	} else {
		report.StopMethod = "kill_fallback"
		if err := command.Process.Kill(); err != nil {
			return report, fmt.Errorf("stop experiment process after CTRL_BREAK failed (%v): %w", stopErr, err)
		}
	}
	select {
	case <-exited:
		report.Stopped = true
		report.GracefulStop = stopErr == nil
	case <-time.After(5 * time.Second):
		if report.StopMethod == "ctrl_break" {
			report.StopMethod = "kill_fallback"
			if err := command.Process.Kill(); err != nil {
				return report, fmt.Errorf("experiment process ignored CTRL_BREAK and kill failed: %w", err)
			}
			select {
			case <-exited:
				report.Stopped = true
			case <-time.After(5 * time.Second):
				return report, fmt.Errorf("experiment process did not exit after kill fallback")
			}
		} else {
			return report, fmt.Errorf("experiment process did not exit")
		}
	}

	report.InterfaceCleaned, report.RoutesCleaned, report.RemainingRoutes = waitForCleanup(interfaceName, expectedPrefix, adapter.Index, 15*time.Second)
	if !resumeDetectedAt.IsZero() {
		report.ResumeActionMillis = time.Since(resumeDetectedAt).Milliseconds()
	}
	if !report.InterfaceCleaned || !report.RoutesCleaned {
		return report, fmt.Errorf("TUN cleanup incomplete: interface_cleaned=%t routes_cleaned=%t", report.InterfaceCleaned, report.RoutesCleaned)
	}
	if probeFailed {
		if len(report.IPv6Probes) > 0 {
			probe := report.IPv6Probes[0]
			return report, fmt.Errorf("IPv6 policy probe failed: dns_blocked=%t dns_addresses=%v dns_error=%q connection_blocked=%t connection_error=%q", probe.DNSBlocked, probe.DNSAddresses, probe.DNSError, probe.ConnectionBlocked, probe.ConnectionError)
		}
		return report, fmt.Errorf("one or more active probes failed")
	}
	return report, nil
}

func unavailableInterface(watched []string) string {
	if len(watched) == 0 {
		return ""
	}
	adapters, err := interfaces.Enumerate()
	if err != nil {
		return ""
	}
	for _, name := range watched {
		available := false
		for _, adapter := range adapters {
			if strings.EqualFold(adapter.FriendlyName, name) && adapter.Status == "up" {
				available = true
				break
			}
		}
		if !available {
			return name
		}
	}
	return ""
}

func runIPv6Probe(ctx context.Context, name, target string) IPv6Probe {
	probe := IPv6Probe{DNSName: name, ConnectionTarget: target}
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip6", name)
	if err != nil {
		probe.DNSBlocked = true
		probe.DNSError = err.Error()
	} else {
		for _, address := range addresses {
			probe.DNSAddresses = append(probe.DNSAddresses, address.String())
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	connection, err := tls.DialWithDialer(dialer, "tcp6", target, &tls.Config{
		ServerName: "cloudflare-dns.com",
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		probe.ConnectionBlocked = true
		probe.ConnectionError = err.Error()
	} else {
		_ = connection.Close()
	}
	return probe
}

func runDNSProbe(ctx context.Context, name string) DNSProbe {
	probe := DNSProbe{Name: name}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, name)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	for _, address := range addresses {
		probe.Addresses = append(probe.Addresses, address.IP.String())
	}
	probe.Resolved = len(probe.Addresses) > 0
	if !probe.Resolved {
		probe.Error = "resolver returned no addresses"
	}
	return probe
}

func runICMPProbe(ctx context.Context, target string) ICMPProbe {
	probe := ICMPProbe{Target: target}
	command := exec.CommandContext(ctx, "ping.exe", "-n", "2", "-w", "2000", target)
	output, err := command.CombinedOutput()
	probe.Output = strings.TrimSpace(string(output))
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	probe.Reached = true
	return probe
}

func runNTPUDPProbe(target string) UDPProbe {
	probe := UDPProbe{Target: target}
	connection, err := net.DialTimeout("udp", target, 5*time.Second)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	defer connection.Close()
	probe.LocalAddress = connection.LocalAddr().String()
	probe.RemoteAddress = connection.RemoteAddr().String()
	_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
	query := make([]byte, 48)
	query[0] = 0x1b // NTPv3 client request.
	if _, err := connection.Write(query); err != nil {
		probe.Error = err.Error()
		return probe
	}
	probe.Sent = true
	response := make([]byte, 1500)
	count, err := connection.Read(response)
	if err != nil {
		probe.Error = err.Error()
		return probe
	}
	probe.ResponseBytes = count
	if count < 48 || response[0]>>3 == 0 {
		probe.Error = "invalid NTP response"
		return probe
	}
	probe.Responded = true
	return probe
}

func waitForTUN(ctx context.Context, exited <-chan error, name, expectedPrefix string, timeout time.Duration) (interfaces.Adapter, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return interfaces.Adapter{}, ctx.Err()
		case err := <-exited:
			return interfaces.Adapter{}, fmt.Errorf("sing-box exited before TUN appeared: %w", err)
		case <-deadline.C:
			return interfaces.Adapter{}, fmt.Errorf("interface %q did not appear", name)
		case <-ticker.C:
			adapters, err := interfaces.Enumerate()
			if err != nil {
				continue
			}
			for _, adapter := range adapters {
				if strings.EqualFold(adapter.FriendlyName, name) && adapterHasPrefix(adapter, expectedPrefix) {
					return adapter, nil
				}
			}
		}
	}
}

func waitForCleanup(name, prefix string, index uint32, timeout time.Duration) (bool, bool, []routes.Route) {
	deadline := time.Now().Add(timeout)
	for {
		interfaceCleaned := true
		if adapters, err := interfaces.Enumerate(); err == nil {
			for _, adapter := range adapters {
				if strings.EqualFold(adapter.FriendlyName, name) && adapterHasPrefix(adapter, prefix) {
					interfaceCleaned = false
				}
			}
		}
		remaining := make([]routes.Route, 0)
		if table, err := routes.Enumerate(); err == nil {
			for _, route := range table {
				if route.InterfaceIndex == index {
					remaining = append(remaining, route)
				}
			}
		}
		if interfaceCleaned && len(remaining) == 0 {
			return true, true, remaining
		}
		if time.Now().After(deadline) {
			return interfaceCleaned, len(remaining) == 0, remaining
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func adapterHasPrefix(adapter interfaces.Adapter, value string) bool {
	wanted, err := netip.ParsePrefix(value)
	if err != nil {
		return false
	}
	for _, address := range adapter.Addresses {
		ip, err := netip.ParseAddr(address.IP)
		if err == nil && ip.BitLen() == wanted.Addr().BitLen() && wanted.Contains(ip) {
			return true
		}
	}
	return false
}
