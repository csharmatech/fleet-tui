package main

import (
	"net"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestThemeCyclingAllSevenThemes(t *testing.T) {
	t.Setenv("FLEET_HOSTS_PATH", "/tmp/test_community_hosts.json")
	defer os.Remove("/tmp/test_community_hosts.json")

	m := initialModel()

	// 1. Initial theme is Cyberpunk (index 0)
	if m.theme().Name != "Cyberpunk" {
		t.Fatalf("expected Cyberpunk, got %s", m.theme().Name)
	}

	expectedThemes := []string{
		"Dracula",
		"Nord",
		"Matrix",
		"Solarized Light",
		"Paper Light",
		"Catppuccin Latte",
		"Cyberpunk",
	}

	for _, expected := range expectedThemes {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
		m = next.(model)
		if m.theme().Name != expected {
			t.Fatalf("expected theme %s, got %s", expected, m.theme().Name)
		}
	}

	// Verify view rendering contains theme pill
	viewStr := m.View()
	if !strings.Contains(viewStr, "[t] Theme: Cyberpunk") {
		t.Fatalf("expected [t] Theme: Cyberpunk in view, got:\n%s", viewStr)
	}
}

func TestSearchFilter(t *testing.T) {
	m := initialModel()
	m.vms = []VM{
		{Name: "web-server-01", IP: "10.0.0.1", Status: "ONLINE"},
		{Name: "db-primary", IP: "10.0.0.2", Status: "ONLINE"},
		{Name: "k8s-node-worker", IP: "10.0.0.3", Status: "SHUT OFF"},
	}

	// Filter for "db"
	m.searchInput.SetValue("db")
	matched := m.filteredVMIndices()
	if len(matched) != 1 || m.vms[matched[0]].Name != "db-primary" {
		t.Fatalf("expected 1 match for db-primary, got %d", len(matched))
	}

	// Filter for "10.0.0.3"
	m.searchInput.SetValue("10.0.0.3")
	matched = m.filteredVMIndices()
	if len(matched) != 1 || m.vms[matched[0]].Name != "k8s-node-worker" {
		t.Fatalf("expected 1 match for 10.0.0.3, got %d", len(matched))
	}
}

func TestTelemetryParsing(t *testing.T) {
	vm := VM{Name: "test-vm"}
	sampleOutput := `Filesystem      Size  Used Avail Use% Mounted on
/dev/sda1        40G   12G   26G  32% /
===MEM===
              total        used        free      shared  buff/cache   available
Mem:           7820        2140        3210          18        2470        5410
Swap:          2048           0        2048
===UPTIME===
 14:02:18 up 12 days,  3:14,  1 user,  load average: 0.14, 0.22, 0.18
`
	parseCombinedTelemetry(&vm, sampleOutput)

	if vm.DiskPercent < 0.31 || vm.DiskPercent > 0.33 {
		t.Errorf("expected disk percent around 0.32, got %f", vm.DiskPercent)
	}
	if vm.DiskUsed != "12G" || vm.DiskTotal != "40G" {
		t.Errorf("unexpected disk stats: %s/%s", vm.DiskUsed, vm.DiskTotal)
	}
	if vm.RAMPercent <= 0 {
		t.Errorf("expected ram percent > 0, got %f", vm.RAMPercent)
	}
	if vm.CPULoad != "0.14, 0.22, 0.18" {
		t.Errorf("unexpected CPU load: %s", vm.CPULoad)
	}
}

func TestLiveStreamDiagnosticToggle(t *testing.T) {
	m := initialModel()
	if m.liveDiag {
		t.Fatal("expected liveDiag to be false initially")
	}

	// Initial view should show STREAM OFF
	viewStr := m.View()
	if !strings.Contains(viewStr, "○ STREAM OFF [l]") {
		t.Fatalf("expected '○ STREAM OFF [l]' in view, got:\n%s", viewStr)
	}

	// Press 'l' to toggle live streaming ON
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = next.(model)
	if !m.liveDiag {
		t.Fatal("expected liveDiag to be true after pressing 'l'")
	}

	viewStr = m.View()
	if !strings.Contains(viewStr, "● LIVE STREAM") {
		t.Fatalf("expected '● LIVE STREAM' in view, got:\n%s", viewStr)
	}

	// Press 'l' again to toggle live streaming OFF
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	m = next.(model)
	if m.liveDiag {
		t.Fatal("expected liveDiag to be false after second 'l'")
	}

	viewStr = m.View()
	if !strings.Contains(viewStr, "○ STREAM OFF [l]") {
		t.Fatalf("expected '○ STREAM OFF [l]' in view, got:\n%s", viewStr)
	}
}


func TestPreflightProbe(t *testing.T) {
	// Test failure on closed / unreachable port
	err := preflightProbe("127.0.0.1:59999")
	if err == nil {
		t.Fatal("expected preflightProbe to fail on unused port 59999, but succeeded")
	}

	// Test success on an active listener
	l, listenErr := net.Listen("tcp", "127.0.0.1:0")
	if listenErr != nil {
		t.Fatalf("failed to open test listener: %v", listenErr)
	}
	defer l.Close()

	if err := preflightProbe(l.Addr().String()); err != nil {
		t.Fatalf("expected preflightProbe to succeed on %s, got: %v", l.Addr().String(), err)
	}
}
