package vbox

import (
	"context"
	"runtime"
	"testing"

	"github.com/tabvm/desktop-agent/internal/models"
	"github.com/tabvm/desktop-agent/internal/runner"
)

func TestParseForwardingRules_SingleNIC(t *testing.T) {
	out := `NIC 1 Rule(0):  name = ssh, protocol = tcp, host ip = 127.0.0.1, host port = 2222, guest ip = , guest port = 22`

	rules := parseForwardingRules(out)
	if len(rules[1]) != 1 {
		t.Fatalf("expected 1 rule on NIC 1, got %d", len(rules[1]))
	}
	r := rules[1][0]
	if r.Name != "ssh" || r.Protocol != "tcp" || r.HostIP != "127.0.0.1" || r.HostPort != 2222 || r.GuestPort != 22 {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestParseForwardingRules_MultiNIC(t *testing.T) {
	out := `NIC 1 Rule(0):  name = ssh, protocol = tcp, host ip = 127.0.0.1, host port = 2222, guest ip = , guest port = 22
NIC 2 Rule(0):  name = web, protocol = tcp, host ip = 127.0.0.1, host port = 8080, guest ip = , guest port = 80`

	rules := parseForwardingRules(out)
	if len(rules[1]) != 1 || len(rules[2]) != 1 {
		t.Fatalf("expected one rule on each of NIC 1 and NIC 2, got %d and %d", len(rules[1]), len(rules[2]))
	}
	if rules[1][0].Name != "ssh" || rules[1][0].HostPort != 2222 {
		t.Fatalf("unexpected NIC 1 rule: %+v", rules[1][0])
	}
	if rules[2][0].Name != "web" || rules[2][0].HostPort != 8080 || rules[2][0].GuestPort != 80 {
		t.Fatalf("unexpected NIC 2 rule: %+v", rules[2][0])
	}
}

func TestParseForwardingRules_Empty(t *testing.T) {
	out := `Name: demo
NIC 1: MAC: 080027AABBCC, Attachment: NAT`
	rules := parseForwardingRules(out)
	if len(rules) != 0 {
		t.Fatalf("expected no rules, got %d NIC entries", len(rules))
	}
}

func TestParseForwardingRules_EmptyHostGuestIP(t *testing.T) {
	out := `NIC 1 Rule(0):  name = dns, protocol = udp, host ip = , host port = 5353, guest ip = , guest port = 53`

	rules := parseForwardingRules(out)
	if len(rules[1]) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules[1]))
	}
	r := rules[1][0]
	if r.HostIP != "" || r.GuestIP != "" {
		t.Fatalf("expected empty host/guest IP, got host=%q guest=%q", r.HostIP, r.GuestIP)
	}
	if r.Protocol != "udp" || r.HostPort != 5353 || r.GuestPort != 53 {
		t.Fatalf("unexpected rule: %+v", r)
	}
}

func TestNatpfAddArgs_LiveAndStopped(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"
	rule := models.PortForwardingRule{Name: "ssh", Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 2222, GuestIP: "", GuestPort: 22}

	live := natpfAddArgs(id, 1, rule, true)
	wantLive := []string{"controlvm", id, "natpf1", "ssh,tcp,127.0.0.1,2222,,22"}
	assertArgs(t, live, wantLive)

	stopped := natpfAddArgs(id, 2, rule, false)
	wantStopped := []string{"modifyvm", id, "--natpf2", "ssh,tcp,127.0.0.1,2222,,22"}
	assertArgs(t, stopped, wantStopped)
}

func TestNatpfDeleteArgs_LiveAndStopped(t *testing.T) {
	id := "11111111-1111-1111-1111-111111111111"

	live := natpfDeleteArgs(id, 1, "ssh", true)
	assertArgs(t, live, []string{"controlvm", id, "natpf1", "delete", "ssh"})

	stopped := natpfDeleteArgs(id, 3, "ssh", false)
	assertArgs(t, stopped, []string{"modifyvm", id, "--natpf3", "delete", "ssh"})
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d mismatch: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestAddPortForwarding_RejectsNonNatSlot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"
nic1="bridged"
macaddress1="0800271122AA"`},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AddPortForwarding(context.Background(), id, models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "tcp", HostPort: 2222, GuestPort: 22,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for non-NAT slot, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_RejectsBadProtocol(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.AddPortForwarding(context.Background(), "11111111-1111-1111-1111-111111111111", models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "sctp", HostPort: 2222, GuestPort: 22,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for bad protocol, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_RejectsBadPort(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.AddPortForwarding(context.Background(), "11111111-1111-1111-1111-111111111111", models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "tcp", HostPort: 70000, GuestPort: 22,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for out-of-range host port, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_RejectsNameWithComma(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.AddPortForwarding(context.Background(), "11111111-1111-1111-1111-111111111111", models.PortForwardingRequest{
		Slot: 1, Name: "ss,h", Protocol: "tcp", HostPort: 2222, GuestPort: 22,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for name with comma, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_RejectsLeadingDashName(t *testing.T) {
	svc := NewService(&fakeRunner{}, Config{})
	_, err := svc.AddPortForwarding(context.Background(), "11111111-1111-1111-1111-111111111111", models.PortForwardingRequest{
		Slot: 1, Name: "-flag", Protocol: "tcp", HostPort: 2222, GuestPort: 22,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for leading-dash name, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_RejectsDuplicateNamePerNic(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"
nic1="nat"
macaddress1="0800271122AA"`},
		path + " showvminfo " + id: {ExitCode: 0, StandardOutput: `NIC 1 Rule(0):  name = ssh, protocol = tcp, host ip = 127.0.0.1, host port = 2222, guest ip = , guest port = 22`},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AddPortForwarding(context.Background(), id, models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "tcp", HostPort: 3333, GuestPort: 22,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for duplicate name on the same NIC, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_RejectsHostPortCollisionAcrossNics(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `VMState="poweroff"
nic1="nat"
macaddress1="0800271122AA"
nic2="nat"
macaddress2="0800271133BB"`},
		// An existing rule on NIC 2 already uses host port 8080/tcp.
		path + " showvminfo " + id: {ExitCode: 0, StandardOutput: `NIC 2 Rule(0):  name = web, protocol = tcp, host ip = 127.0.0.1, host port = 8080, guest ip = , guest port = 80`},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AddPortForwarding(context.Background(), id, models.PortForwardingRequest{
		Slot: 1, Name: "alt", Protocol: "tcp", HostPort: 8080, GuestPort: 8080,
	})
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError for host port collision across NICs, got %T: %v", err, err)
	}
}

func TestAddPortForwarding_DefaultsHostIPToLoopback(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable":                 {{ExitCode: 0, StandardOutput: "VMState=\"poweroff\"\nnic1=\"nat\"\nmacaddress1=\"0800271122AA\""}},
		path + " showvminfo " + id:                                        {{ExitCode: 0, StandardOutput: ""}},
		path + " modifyvm " + id + " --natpf1 ssh,tcp,127.0.0.1,2222,,22": {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	_, err := svc.AddPortForwarding(context.Background(), id, models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "tcp", HostIP: "", HostPort: 2222, GuestPort: 22,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !run.issued("--natpf1 ssh,tcp,127.0.0.1,2222,,22") {
		t.Fatalf("expected empty host IP to default to 127.0.0.1 in the rule spec; calls: %v", run.calls)
	}
}

func TestAddPortForwarding_StoppedUsesModifyvm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable":                 {{ExitCode: 0, StandardOutput: "VMState=\"poweroff\"\nnic1=\"nat\"\nmacaddress1=\"0800271122AA\""}},
		path + " showvminfo " + id:                                        {{ExitCode: 0, StandardOutput: ""}},
		path + " modifyvm " + id + " --natpf1 ssh,tcp,127.0.0.1,2222,,22": {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AddPortForwarding(context.Background(), id, models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 2222, GuestPort: 22,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if run.issued("controlvm") {
		t.Fatal("stopped VM must use modifyvm, not controlvm")
	}
}

func TestAddPortForwarding_LiveUsesControlvm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable":                {{ExitCode: 0, StandardOutput: "VMState=\"running\"\nnic1=\"nat\"\nmacaddress1=\"0800271122AA\""}},
		path + " showvminfo " + id:                                       {{ExitCode: 0, StandardOutput: ""}},
		path + " controlvm " + id + " natpf1 ssh,tcp,127.0.0.1,2222,,22": {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.AddPortForwarding(context.Background(), id, models.PortForwardingRequest{
		Slot: 1, Name: "ssh", Protocol: "tcp", HostIP: "127.0.0.1", HostPort: 2222, GuestPort: 22,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if run.issued("modifyvm " + id + " --natpf1") {
		t.Fatal("running VM must use controlvm, not modifyvm")
	}
}

func TestDeletePortForwarding_StoppedUsesModifyvm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable": {{ExitCode: 0, StandardOutput: "VMState=\"poweroff\""}},
		path + " modifyvm " + id + " --natpf1 delete ssh": {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.DeletePortForwarding(context.Background(), id, 1, "ssh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if !run.issued("modifyvm " + id + " --natpf1 delete ssh") {
		t.Fatalf("expected modifyvm delete; calls: %v", run.calls)
	}
}

func TestDeletePortForwarding_LiveUsesControlvm(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &queuedRunner{queues: map[string][]runner.Result{
		path + " --version": {{ExitCode: 0, StandardOutput: "7.2.12r174389\n"}},
		path + " showvminfo " + id + " --machinereadable": {{ExitCode: 0, StandardOutput: "VMState=\"running\""}},
		path + " controlvm " + id + " natpf1 delete ssh":  {{ExitCode: 0}},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.DeletePortForwarding(context.Background(), id, 1, "ssh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got %+v", resp)
	}
	if !run.issued("controlvm " + id + " natpf1 delete ssh") {
		t.Fatalf("expected controlvm delete; calls: %v", run.calls)
	}
}

func TestNetworkOptions_AttachesForwardingRules(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("VirtualBox discovery is Windows-only in this test")
	}

	id := "11111111-1111-1111-1111-111111111111"
	path := createTempExecutable(t)
	run := &fakeRunner{results: map[string]runner.Result{
		path + " --version": {ExitCode: 0, StandardOutput: "7.2.12r174389\n"},
		path + " showvminfo " + id + " --machinereadable": {ExitCode: 0, StandardOutput: `nic1="nat"
macaddress1="0800271122AA"
nic2="nat"
macaddress2="0800271133BB"`},
		path + " showvminfo " + id: {ExitCode: 0, StandardOutput: `NIC 1 Rule(0):  name = ssh, protocol = tcp, host ip = 127.0.0.1, host port = 2222, guest ip = , guest port = 22
NIC 2 Rule(0):  name = web, protocol = tcp, host ip = 127.0.0.1, host port = 8080, guest ip = , guest port = 80`},
	}}

	svc := NewService(run, Config{CandidatePaths: []string{path}})
	resp, err := svc.NetworkOptions(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Adapters) != 2 {
		t.Fatalf("expected 2 adapters, got %d", len(resp.Adapters))
	}
	if len(resp.Adapters[0].Forwarding) != 1 || resp.Adapters[0].Forwarding[0].Name != "ssh" {
		t.Fatalf("expected NIC 1 to carry the ssh rule, got %+v", resp.Adapters[0].Forwarding)
	}
	if len(resp.Adapters[1].Forwarding) != 1 || resp.Adapters[1].Forwarding[0].HostPort != 8080 {
		t.Fatalf("expected NIC 2 to carry the web rule, got %+v", resp.Adapters[1].Forwarding)
	}
}
