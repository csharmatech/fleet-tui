package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/crypto/ssh"
)

type UserSettings struct {
	RefreshInterval int    `json:"refresh_interval"`
	Theme           string `json:"theme,omitempty"`
}

type UITheme struct {
	Name       string
	IsLight    bool
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Background lipgloss.Color
	HeaderBg   lipgloss.Color
	Line       lipgloss.Color
	Text       lipgloss.Color
	Muted      lipgloss.Color
	Success    lipgloss.Color
	Warning    lipgloss.Color
	Danger     lipgloss.Color
	Highlight  lipgloss.Color
}

var AvailableThemes = []UITheme{
	{
		Name:       "Cyberpunk",
		IsLight:    false,
		Primary:    lipgloss.Color("#22d3ee"),
		Secondary:  lipgloss.Color("#c084fc"),
		Background: lipgloss.Color("#111827"),
		HeaderBg:   lipgloss.Color("#0f172a"),
		Line:       lipgloss.Color("#334155"),
		Text:       lipgloss.Color("#cbd5e1"),
		Muted:      lipgloss.Color("#94a3b8"),
		Success:    lipgloss.Color("#34d399"),
		Warning:    lipgloss.Color("#fbbf24"),
		Danger:     lipgloss.Color("#fb7185"),
		Highlight:  lipgloss.Color("#1e293b"),
	},
	{
		Name:       "Dracula",
		IsLight:    false,
		Primary:    lipgloss.Color("#bd93f9"),
		Secondary:  lipgloss.Color("#ff79c6"),
		Background: lipgloss.Color("#282a36"),
		HeaderBg:   lipgloss.Color("#1e1f29"),
		Line:       lipgloss.Color("#44475a"),
		Text:       lipgloss.Color("#f8f8f2"),
		Muted:      lipgloss.Color("#6272a4"),
		Success:    lipgloss.Color("#50fa7b"),
		Warning:    lipgloss.Color("#f1fa8c"),
		Danger:     lipgloss.Color("#ff5555"),
		Highlight:  lipgloss.Color("#44475a"),
	},
	{
		Name:       "Nord",
		IsLight:    false,
		Primary:    lipgloss.Color("#88c0d0"),
		Secondary:  lipgloss.Color("#81a1c1"),
		Background: lipgloss.Color("#2e3440"),
		HeaderBg:   lipgloss.Color("#242933"),
		Line:       lipgloss.Color("#4c566a"),
		Text:       lipgloss.Color("#eceff4"),
		Muted:      lipgloss.Color("#d8dee9"),
		Success:    lipgloss.Color("#a3be8c"),
		Warning:    lipgloss.Color("#ebcb8b"),
		Danger:     lipgloss.Color("#bf616a"),
		Highlight:  lipgloss.Color("#3b4252"),
	},
	{
		Name:       "Matrix",
		IsLight:    false,
		Primary:    lipgloss.Color("#22c55e"),
		Secondary:  lipgloss.Color("#a3e635"),
		Background: lipgloss.Color("#09090b"),
		HeaderBg:   lipgloss.Color("#000000"),
		Line:       lipgloss.Color("#1e293b"),
		Text:       lipgloss.Color("#f4f4f5"),
		Muted:      lipgloss.Color("#71717a"),
		Success:    lipgloss.Color("#4ade80"),
		Warning:    lipgloss.Color("#eab308"),
		Danger:     lipgloss.Color("#ef4444"),
		Highlight:  lipgloss.Color("#14532d"),
	},
	{
		Name:       "Solarized Light",
		IsLight:    true,
		Primary:    lipgloss.Color("#268bd2"),
		Secondary:  lipgloss.Color("#b58900"),
		Background: lipgloss.Color("#fdf6e3"),
		HeaderBg:   lipgloss.Color("#eee8d5"),
		Line:       lipgloss.Color("#93a1a1"),
		Text:       lipgloss.Color("#073642"),
		Muted:      lipgloss.Color("#586e75"),
		Success:    lipgloss.Color("#859900"),
		Warning:    lipgloss.Color("#cb4b16"),
		Danger:     lipgloss.Color("#dc322f"),
		Highlight:  lipgloss.Color("#eee8d5"),
	},
	{
		Name:       "Paper Light",
		IsLight:    true,
		Primary:    lipgloss.Color("#0284c7"),
		Secondary:  lipgloss.Color("#7c3aed"),
		Background: lipgloss.Color("#f8fafc"),
		HeaderBg:   lipgloss.Color("#e2e8f0"),
		Line:       lipgloss.Color("#cbd5e1"),
		Text:       lipgloss.Color("#0f172a"),
		Muted:      lipgloss.Color("#64748b"),
		Success:    lipgloss.Color("#16a34a"),
		Warning:    lipgloss.Color("#d97706"),
		Danger:     lipgloss.Color("#dc2626"),
		Highlight:  lipgloss.Color("#e0f2fe"),
	},
	{
		Name:       "Catppuccin Latte",
		IsLight:    true,
		Primary:    lipgloss.Color("#1e66f5"),
		Secondary:  lipgloss.Color("#8839ef"),
		Background: lipgloss.Color("#eff1f5"),
		HeaderBg:   lipgloss.Color("#e6e9ef"),
		Line:       lipgloss.Color("#bcc0cc"),
		Text:       lipgloss.Color("#4c4f69"),
		Muted:      lipgloss.Color("#7c7f93"),
		Success:    lipgloss.Color("#40a02b"),
		Warning:    lipgloss.Color("#df8e1d"),
		Danger:     lipgloss.Color("#d20f39"),
		Highlight:  lipgloss.Color("#ccd0da"),
	},
}

type InbuiltCmd struct {
	Label       string
	Command     string
	Description string
	Icon        string
}

type HostConfig struct {
	Name    string `json:"name"`
	IP      string `json:"ip"`
	User    string `json:"user"`
	IsDemo  bool   `json:"is_demo,omitempty"`
	IsLocal bool   `json:"is_local,omitempty"`
}

type VM struct {
	Name        string
	IP          string
	User        string
	IsDemo      bool
	IsLocal     bool
	DiskPercent float64
	DiskUsed    string
	DiskTotal   string
	DiskAvail   string
	RAMPercent  float64
	RAMUsed     string
	RAMTotal    string
	RAMAvail    string
	CPULoad     string
	Status      string
	LastRTT     time.Duration
}

type tickMsg struct {
	time time.Time
	id   uint64
}

type singleExecMsg struct {
	vmIndex   int
	cmdString string
	stdout    string
	duration  time.Duration
	refreshed time.Time
	err       error
}

type fleetScanMsg struct {
	results []VM
	scanAt  time.Time
}

type model struct {
	vms         []VM
	selectedVM  int
	commands    []InbuiltCmd
	selectedCmd int

	dropdownOpen bool
	dropdownIdx  int

	addModalOpen bool
	addFocusIdx  int
	inputs       []textinput.Model
	modalError   string

	searchActive bool
	searchInput  textinput.Model

	autoRefresh     bool
	refreshInterval int
	tickID          uint64
	liveDiag        bool

	currentThemeIdx int

	viewport     viewport.Model
	spinner      spinner.Model
	diskProgress progress.Model
	ramProgress  progress.Model

	cmdRunning      bool
	lastHeartbeat   time.Time
	lastCmdRun      time.Time
	activeOutput    string
	width           int
	height          int
	remoteDaemonURL string
}

func resolveInventoryPath() string {
	if p := os.Getenv("FLEET_HOSTS_PATH"); p != "" {
		return p
	}
	if _, err := os.Stat("hosts.json"); err == nil {
		return "hosts.json"
	}
	home, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(home, ".config", "fleet-tui", "hosts.json")
	}
	return "hosts.json"
}

func resolveSettingsPath() string {
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, ".config", "fleet-tui")
		_ = os.MkdirAll(dir, 0755)
		return filepath.Join(dir, "settings.json")
	}
	return "settings.json"
}

func loadUserSettings() UserSettings {
	p := resolveSettingsPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return UserSettings{RefreshInterval: 2, Theme: "Cyberpunk"}
	}
	var s UserSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return UserSettings{RefreshInterval: 2, Theme: "Cyberpunk"}
	}
	if s.RefreshInterval <= 0 {
		s.RefreshInterval = 2
	}
	return s
}

func saveUserSettings(s UserSettings) {
	p := resolveSettingsPath()
	data, err := json.MarshalIndent(s, "", "  ")
	if err == nil {
		_ = os.WriteFile(p, data, 0644)
	}
}

func loadInventoryConfigs() []HostConfig {
	p := resolveInventoryPath()
	var configs []HostConfig
	data, err := os.ReadFile(p)
	if err != nil {
		sampleData, sampleErr := os.ReadFile("hosts.json.example")
		if sampleErr == nil {
			_ = json.Unmarshal(sampleData, &configs)
			_ = saveInventory(configs)
		} else {
			configs = []HostConfig{
				{Name: "localhost", IP: "127.0.0.1", User: "local", IsLocal: true},
			}
			_ = saveInventory(configs)
		}
	} else {
		_ = json.Unmarshal(data, &configs)
	}

	sort.Slice(configs, func(i, j int) bool {
		return strings.ToLower(configs[i].Name) < strings.ToLower(configs[j].Name)
	})
	return configs
}

func loadInventory() []VM {
	configs := loadInventoryConfigs()
	vms := make([]VM, len(configs))
	for i, c := range configs {
		vms[i] = VM{
			Name:    c.Name,
			IP:      c.IP,
			User:    c.User,
			IsDemo:  c.IsDemo,
			IsLocal: c.IsLocal || c.IP == "127.0.0.1" || c.IP == "localhost",
			Status:  "PROBING",
		}
	}
	return vms
}

func saveInventory(configs []HostConfig) error {
	p := resolveInventoryPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(configs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

func findSSHKey() []byte {
	if p := os.Getenv("FLEET_SSH_KEY"); p != "" {
		if b, err := os.ReadFile(p); err == nil {
			return b
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}
	for _, c := range candidates {
		if b, err := os.ReadFile(c); err == nil {
			return b
		}
	}
	return nil
}

func doTick(intervalSeconds int, id uint64) tea.Cmd {
	return tea.Tick(time.Duration(intervalSeconds)*time.Second, func(t time.Time) tea.Msg {
		return tickMsg{time: t, id: id}
	})
}

func runCommand(vm VM, vmIdx int, cmd InbuiltCmd) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		if vm.IsDemo {
			return singleExecMsg{
				vmIndex:   vmIdx,
				cmdString: cmd.Command,
				stdout:    fmt.Sprintf("// Simulated output for Demo host %s (%s)\n[OK] %s executed cleanly in 1ms.", vm.Name, vm.IP, cmd.Label),
				duration:  time.Millisecond,
				refreshed: time.Now(),
				err:       nil,
			}
		}

		if vm.IsLocal {
			c := exec.Command("bash", "-c", cmd.Command)
			out, err := c.CombinedOutput()
			return singleExecMsg{
				vmIndex:   vmIdx,
				cmdString: cmd.Command,
				stdout:    string(out),
				duration:  time.Since(start),
				refreshed: time.Now(),
				err:       err,
			}
		}

		conn, err := net.DialTimeout("tcp", vm.IP+":22", 600*time.Millisecond)
		if err != nil {
			return singleExecMsg{
				vmIndex:   vmIdx,
				cmdString: cmd.Command,
				err:       fmt.Errorf("host %s (%s) is SHUT OFF or UNREACHABLE", vm.Name, vm.IP),
				refreshed: time.Now(),
			}
		}
		conn.Close()

		key := findSSHKey()
		if key == nil {
			return singleExecMsg{vmIndex: vmIdx, cmdString: cmd.Command, err: fmt.Errorf("no SSH private key found in ~/.ssh/"), refreshed: time.Now()}
		}

		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return singleExecMsg{vmIndex: vmIdx, cmdString: cmd.Command, err: fmt.Errorf("parse SSH key error: %w", err), refreshed: time.Now()}
		}

		config := &ssh.ClientConfig{
			User:            vm.User,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			Timeout:         2 * time.Second,
		}

		client, err := ssh.Dial("tcp", vm.IP+":22", config)
		if err != nil {
			return singleExecMsg{vmIndex: vmIdx, cmdString: cmd.Command, err: fmt.Errorf("ssh dial error: %w", err), refreshed: time.Now()}
		}
		defer client.Close()

		session, err := client.NewSession()
		if err != nil {
			return singleExecMsg{vmIndex: vmIdx, cmdString: cmd.Command, err: fmt.Errorf("session error: %w", err), refreshed: time.Now()}
		}
		defer session.Close()

		out, err := session.CombinedOutput(cmd.Command)
		return singleExecMsg{
			vmIndex:   vmIdx,
			cmdString: cmd.Command,
			stdout:    string(out),
			duration:  time.Since(start),
			refreshed: time.Now(),
			err:       err,
		}
	}
}

func performScan(vms []VM) []VM {
	if len(vms) == 0 {
		return []VM{}
	}

	updated := make([]VM, len(vms))
	copy(updated, vms)

	var wg sync.WaitGroup
	key := findSSHKey()
	scanScript := "df -Ph /; echo '===MEM===' ; free -m; echo '===UPTIME===' ; uptime"

	for i := range updated {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			tStart := time.Now()

			if updated[idx].IsDemo {
				updated[idx].Status = "DEMO"
				updated[idx].DiskUsed = "-"
				updated[idx].DiskTotal = "-"
				updated[idx].RAMUsed = "-"
				updated[idx].RAMTotal = "-"
				updated[idx].CPULoad = "-"
				return
			}

			if updated[idx].IsLocal {
				out, err := exec.Command("bash", "-c", scanScript).CombinedOutput()
				if err == nil {
					parseCombinedTelemetry(&updated[idx], string(out))
					updated[idx].Status = "ONLINE"
					updated[idx].LastRTT = time.Since(tStart)
				} else {
					updated[idx].Status = "ERROR"
				}
				return
			}

			conn, err := net.DialTimeout("tcp", updated[idx].IP+":22", 600*time.Millisecond)
			if err != nil {
				updated[idx].Status = "SHUT OFF"
				updated[idx].DiskPercent = 0.0
				updated[idx].DiskUsed = "-"
				updated[idx].DiskTotal = "-"
				updated[idx].RAMPercent = 0.0
				updated[idx].RAMUsed = "-"
				updated[idx].RAMTotal = "-"
				updated[idx].CPULoad = "-"
				updated[idx].LastRTT = 0
				return
			}
			conn.Close()

			if key == nil {
				updated[idx].Status = "KEY_ERR"
				return
			}

			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				updated[idx].Status = "KEY_ERR"
				return
			}

			config := &ssh.ClientConfig{
				User:            updated[idx].User,
				Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
				Timeout:         1800 * time.Millisecond,
			}

			client, err := ssh.Dial("tcp", updated[idx].IP+":22", config)
			if err != nil {
				updated[idx].Status = "SSH_ERR"
				return
			}
			defer client.Close()

			session, err := client.NewSession()
			if err != nil {
				updated[idx].Status = "SESS_ERR"
				return
			}
			defer session.Close()

			var b bytes.Buffer
			session.Stdout = &b
			if err := session.Run(scanScript); err == nil {
				parseCombinedTelemetry(&updated[idx], b.String())
				updated[idx].Status = "ONLINE"
				updated[idx].LastRTT = time.Since(tStart)
			} else {
				updated[idx].Status = "CMD_ERR"
			}
		}(i)
	}
	wg.Wait()
	return updated
}

func scanAllVMs(vms []VM) tea.Cmd {
	return func() tea.Msg {
		results := performScan(vms)
		return fleetScanMsg{results: results, scanAt: time.Now()}
	}
}

func (m model) scanFleet() tea.Cmd {
	if m.remoteDaemonURL != "" {
		url := m.remoteDaemonURL
		return func() tea.Msg {
			client := http.Client{Timeout: 1500 * time.Millisecond}
			resp, err := client.Get(url + "/api/v1/telemetry")
			if err != nil {
				return fleetScanMsg{results: m.vms, scanAt: time.Now()}
			}
			defer resp.Body.Close()
			var fetched []VM
			if err := json.NewDecoder(resp.Body).Decode(&fetched); err == nil && len(fetched) > 0 {
				return fleetScanMsg{results: fetched, scanAt: time.Now()}
			}
			return fleetScanMsg{results: m.vms, scanAt: time.Now()}
		}
	}
	return scanAllVMs(m.vms)
}

func parseCombinedTelemetry(vm *VM, output string) {
	parts := strings.Split(output, "===UPTIME===")
	if len(parts) > 1 {
		uptimeContent := parts[1]
		if idx := strings.Index(uptimeContent, "load average:"); idx != -1 {
			vm.CPULoad = strings.TrimSpace(uptimeContent[idx+13:])
		}
	}

	memParts := strings.Split(parts[0], "===MEM===")
	if len(memParts) > 1 {
		dfContent := memParts[0]
		memContent := memParts[1]

		for _, line := range strings.Split(dfContent, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 6 && fields[5] == "/" {
				vm.DiskTotal = fields[1]
				vm.DiskUsed = fields[2]
				vm.DiskAvail = fields[3]
				pctStr := strings.TrimSuffix(fields[4], "%")
				if val, err := strconv.ParseFloat(pctStr, 64); err == nil {
					vm.DiskPercent = val / 100.0
				}
			}
		}

		for _, line := range strings.Split(memContent, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "Mem:") {
				fields := strings.Fields(trimmed)
				if len(fields) >= 7 {
					totalMB, _ := strconv.ParseFloat(fields[1], 64)
					availMB, _ := strconv.ParseFloat(fields[6], 64)
					if totalMB > 0 {
						usedMB := totalMB - availMB
						vm.RAMPercent = usedMB / totalMB
						vm.RAMTotal = formatMB(totalMB)
						vm.RAMUsed = formatMB(usedMB)
						vm.RAMAvail = formatMB(availMB)
					}
				}
			}
		}
	}
}

func formatMB(mb float64) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1fG", mb/1024.0)
	}
	return fmt.Sprintf("%.0fM", mb)
}

func initialModel(remoteURLs ...string) model {
	remoteURL := ""
	if len(remoteURLs) > 0 {
		remoteURL = remoteURLs[0]
	}

	vms := loadInventory()
	if remoteURL != "" {
		client := http.Client{Timeout: 1500 * time.Millisecond}
		resp, err := client.Get(remoteURL + "/api/v1/telemetry")
		if err == nil {
			var fetched []VM
			if decodeErr := json.NewDecoder(resp.Body).Decode(&fetched); decodeErr == nil && len(fetched) > 0 {
				vms = fetched
			}
			resp.Body.Close()
		}
	}

	settings := loadUserSettings()

	commands := []InbuiltCmd{
		{Icon: "💾", Label: "Root Disk Usage", Command: "df -Ph /", Description: "POSIX root mount capacity & free blocks"},
		{Icon: "🧠", Label: "RAM & Swap Utilization", Command: "free -m -h", Description: "Physical memory, buffers, cache & swap"},
		{Icon: "⚡", Label: "CPU Total & Load Avg", Command: "top -bn1 | head -n 12", Description: "Kernel tasks, CPU states & core load"},
		{Icon: "🔥", Label: "Top 6 CPU Processes", Command: "ps aux --sort=-%cpu | head -n 7", Description: "Processes consuming highest processor time"},
		{Icon: "📁", Label: "Root Inode Status", Command: "df -i /", Description: "Filesystem index node metadata exhaustion"},
		{Icon: "🚨", Label: "Failed Systemd Daemons", Command: "systemctl --failed", Description: "Units in degraded or error state"},
		{Icon: "🌐", Label: "Listening Ports & Sockets", Command: "ss -tulpn", Description: "Active TCP/UDP network sockets"},
	}

	themeIdx := 0
	for idx, th := range AvailableThemes {
		if strings.EqualFold(th.Name, settings.Theme) {
			themeIdx = idx
			break
		}
	}
	activeTh := AvailableThemes[themeIdx]

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(activeTh.Primary)

	diskProg := progress.New(progress.WithGradient(string(activeTh.Success), string(activeTh.Primary)), progress.WithWidth(28))
	ramProg := progress.New(progress.WithGradient(string(activeTh.Secondary), string(activeTh.Primary)), progress.WithWidth(28))

	vp := viewport.New(80, 5)
	if len(vms) == 0 {
		vp.SetContent("// Inventory is currently empty.\n// Press [n] or [+] to add your first Linux host.\n// Required details: Host Name, IP address, SSH User.")
	} else {
		vp.SetContent("// Connecting to initial VM...\n// Command output will auto-refresh according to interval.")
	}

	inputs := make([]textinput.Model, 3)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "e.g. web-server or k8s-master"
	inputs[0].Focus()
	inputs[0].CharLimit = 28

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "e.g. 192.168.1.100 or 127.0.0.1"
	inputs[1].CharLimit = 40

	inputs[2] = textinput.New()
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		currentUser = "root"
	}
	inputs[2].Placeholder = "e.g. " + currentUser
	inputs[2].SetValue(currentUser)
	inputs[2].CharLimit = 24

	searchInput := textinput.New()
	searchInput.Placeholder = "type to filter hosts..."
	searchInput.CharLimit = 32

	return model{
		vms:             vms,
		selectedVM:      0,
		commands:        commands,
		selectedCmd:     0,
		dropdownOpen:    false,
		dropdownIdx:     0,
		addModalOpen:    false,
		addFocusIdx:     0,
		inputs:          inputs,
		modalError:      "",
		searchActive:    false,
		searchInput:     searchInput,
		autoRefresh:     true,
		refreshInterval: settings.RefreshInterval,
		tickID:          1,
		liveDiag:        false,
		currentThemeIdx: themeIdx,
		spinner:         sp,
		diskProgress:    diskProg,
		ramProgress:     ramProg,
		viewport:        vp,
		cmdRunning:      false,
		lastHeartbeat:   time.Now(),
		lastCmdRun:      time.Now(),
		remoteDaemonURL: remoteURL,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		m.spinner.Tick,
		doTick(m.refreshInterval, m.tickID),
		m.scanFleet(),
	}
	if len(m.vms) > 0 {
		cmds = append(cmds, runCommand(m.vms[0], 0, m.commands[0]))
	}
	return tea.Batch(cmds...)
}

func (m model) filteredVMIndices() []int {
	query := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	var indices []int
	for i, v := range m.vms {
		if query == "" {
			indices = append(indices, i)
			continue
		}
		if strings.Contains(strings.ToLower(v.Name), query) ||
			strings.Contains(strings.ToLower(v.IP), query) ||
			strings.Contains(strings.ToLower(v.Status), query) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 6
		if m.viewport.Width < 40 {
			m.viewport.Width = 40
		}
		return m, nil

	case tickMsg:
		if msg.id != m.tickID {
			return m, nil
		}
		if !m.autoRefresh {
			return m, nil
		}
		cmds := []tea.Cmd{
			doTick(m.refreshInterval, m.tickID),
			m.scanFleet(),
		}
		if m.liveDiag && !m.cmdRunning && len(m.vms) > 0 && m.selectedVM < len(m.vms) {
			vm := m.vms[m.selectedVM]
			if vm.Status != "SHUT OFF" {
				m.cmdRunning = true
				cmds = append(cmds, m.spinner.Tick, runCommand(vm, m.selectedVM, m.commands[m.selectedCmd]))
			}
		}
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		if m.searchActive {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.searchActive = false
				m.searchInput.Blur()
				return m, nil
			case "ctrl+u":
				m.searchInput.Reset()
				return m, nil
			case "enter":
				m.searchActive = false
				m.searchInput.Blur()
				return m, nil
			case "up":
				matched := m.filteredVMIndices()
				for pos, idx := range matched {
					if idx == m.selectedVM && pos > 0 {
						m.selectedVM = matched[pos-1]
						break
					}
				}
				return m, nil
			case "down":
				matched := m.filteredVMIndices()
				for pos, idx := range matched {
					if idx == m.selectedVM && pos < len(matched)-1 {
						m.selectedVM = matched[pos+1]
						break
					}
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				matched := m.filteredVMIndices()
				if len(matched) > 0 {
					found := false
					for _, idx := range matched {
						if idx == m.selectedVM {
							found = true
							break
						}
					}
					if !found {
						m.selectedVM = matched[0]
					}
				}
				return m, cmd
			}
		}

		if m.addModalOpen {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			case "esc":
				m.addModalOpen = false
				m.modalError = ""
				return m, nil
			case "tab", "down":
				m.inputs[m.addFocusIdx].Blur()
				m.addFocusIdx = (m.addFocusIdx + 1) % len(m.inputs)
				m.inputs[m.addFocusIdx].Focus()
				return m, nil
			case "shift+tab", "up":
				m.inputs[m.addFocusIdx].Blur()
				m.addFocusIdx = (m.addFocusIdx - 1 + len(m.inputs)) % len(m.inputs)
				m.inputs[m.addFocusIdx].Focus()
				return m, nil
			case "enter":
				if m.addFocusIdx < len(m.inputs)-1 {
					m.inputs[m.addFocusIdx].Blur()
					m.addFocusIdx++
					m.inputs[m.addFocusIdx].Focus()
					return m, nil
				}

				name := strings.TrimSpace(m.inputs[0].Value())
				ip := strings.TrimSpace(m.inputs[1].Value())
				user := strings.TrimSpace(m.inputs[2].Value())

				if name == "" || ip == "" {
					m.modalError = "Error: Host Name and IP Address cannot be blank."
					return m, nil
				}
				if user == "" {
					user = "root"
				}

				newVM := VM{
					Name:    name,
					IP:      ip,
					User:    user,
					IsLocal: ip == "127.0.0.1" || ip == "localhost",
					Status:  "PROBING",
				}
				m.vms = append(m.vms, newVM)
				sort.Slice(m.vms, func(i, j int) bool {
					return strings.ToLower(m.vms[i].Name) < strings.ToLower(m.vms[j].Name)
				})

				if m.remoteDaemonURL != "" {
					go func(cfg HostConfig, daemonURL string) {
						payload, _ := json.Marshal(cfg)
						client := http.Client{Timeout: 2 * time.Second}
						_, _ = client.Post(daemonURL+"/api/v1/hosts", "application/json", bytes.NewReader(payload))
					}(HostConfig{Name: name, IP: ip, User: user, IsLocal: ip == "127.0.0.1" || ip == "localhost"}, m.remoteDaemonURL)
				} else {
					configs := make([]HostConfig, len(m.vms))
					for i, v := range m.vms {
						configs[i] = HostConfig{
							Name:    v.Name,
							IP:      v.IP,
							User:    v.User,
							IsLocal: v.IsLocal,
							IsDemo:  v.IsDemo,
						}
					}
					_ = saveInventory(configs)
				}

				m.addModalOpen = false
				m.modalError = ""
				m.inputs[0].Reset()
				m.inputs[1].Reset()
				m.addFocusIdx = 0

				for i, v := range m.vms {
					if v.Name == name {
						m.selectedVM = i
						break
					}
				}
				return m, m.scanFleet()
			}
			var cmd tea.Cmd
			m.inputs[m.addFocusIdx], cmd = m.inputs[m.addFocusIdx].Update(msg)
			return m, cmd
		}

		if m.dropdownOpen {
			switch msg.String() {
			case "esc", "q":
				m.dropdownOpen = false
				return m, nil
			case "up", "k":
				if m.dropdownIdx > 0 {
					m.dropdownIdx--
				}
			case "down", "j":
				if m.dropdownIdx < len(m.commands)-1 {
					m.dropdownIdx++
				}
			case "enter":
				m.selectedCmd = m.dropdownIdx
				m.dropdownOpen = false
				if len(m.vms) > 0 && m.selectedVM < len(m.vms) {
					vm := m.vms[m.selectedVM]
					if vm.Status != "SHUT OFF" {
						m.cmdRunning = true
						return m, tea.Batch(
							m.spinner.Tick,
							runCommand(vm, m.selectedVM, m.commands[m.selectedCmd]),
						)
					}
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			if m.searchInput.Value() != "" {
				m.searchInput.Reset()
				return m, nil
			}
		case "/", "f", "F", "ctrl+f":
			m.searchActive = true
			m.searchInput.Focus()
			return m, nil
		case "up", "k":
			matched := m.filteredVMIndices()
			for pos, idx := range matched {
				if idx == m.selectedVM && pos > 0 {
					m.selectedVM = matched[pos-1]
					break
				}
			}
			if m.liveDiag && len(m.vms) > 0 && m.selectedVM < len(m.vms) {
				vm := m.vms[m.selectedVM]
				if vm.Status != "SHUT OFF" {
					m.cmdRunning = true
					return m, tea.Batch(
						m.spinner.Tick,
						runCommand(vm, m.selectedVM, m.commands[m.selectedCmd]),
					)
				}
			}
			return m, nil
		case "down", "j":
			matched := m.filteredVMIndices()
			for pos, idx := range matched {
				if idx == m.selectedVM && pos < len(matched)-1 {
					m.selectedVM = matched[pos+1]
					break
				}
			}
			if m.liveDiag && len(m.vms) > 0 && m.selectedVM < len(m.vms) {
				vm := m.vms[m.selectedVM]
				if vm.Status != "SHUT OFF" {
					m.cmdRunning = true
					return m, tea.Batch(
						m.spinner.Tick,
						runCommand(vm, m.selectedVM, m.commands[m.selectedCmd]),
					)
				}
			}
			return m, nil
		case "n", "+":
			m.addModalOpen = true
			m.addFocusIdx = 0
			m.modalError = ""
			m.inputs[0].Focus()
			return m, nil
		case "x", "delete":
			if len(m.vms) > 0 && m.selectedVM < len(m.vms) {
				deletedName := m.vms[m.selectedVM].Name
				m.vms = append(m.vms[:m.selectedVM], m.vms[m.selectedVM+1:]...)
				if m.selectedVM >= len(m.vms) && len(m.vms) > 0 {
					m.selectedVM = len(m.vms) - 1
				}
				if m.remoteDaemonURL != "" {
					go func(name, daemonURL string) {
						client := http.Client{Timeout: 2 * time.Second}
						_, _ = client.Post(daemonURL+"/api/v1/hosts/delete?name="+url.QueryEscape(name), "text/plain", nil)
					}(deletedName, m.remoteDaemonURL)
				} else {
					configs := make([]HostConfig, len(m.vms))
					for i, v := range m.vms {
						configs[i] = HostConfig{
							Name:    v.Name,
							IP:      v.IP,
							User:    v.User,
							IsLocal: v.IsLocal,
							IsDemo:  v.IsDemo,
						}
					}
					_ = saveInventory(configs)
				}
				if len(m.vms) == 0 {
					m.viewport.SetContent("// All hosts removed.\n// Press [n] or [+] to add a Linux host to monitor.")
				}
				return m, m.scanFleet()
			}
		case "d", "c":
			m.dropdownOpen = true
			m.dropdownIdx = m.selectedCmd
			return m, nil
		case "tab":
			m.selectedCmd = (m.selectedCmd + 1) % len(m.commands)
			if len(m.vms) > 0 && m.selectedVM < len(m.vms) {
				vm := m.vms[m.selectedVM]
				if vm.Status != "SHUT OFF" {
					m.cmdRunning = true
					return m, runCommand(vm, m.selectedVM, m.commands[m.selectedCmd])
				}
			}
		case "i", "R":
			intervals := []int{2, 10, 30, 60}
			for idx, val := range intervals {
				if val == m.refreshInterval {
					m.refreshInterval = intervals[(idx+1)%len(intervals)]
					break
				}
			}
			m.autoRefresh = true
			m.tickID++
			saveUserSettings(UserSettings{RefreshInterval: m.refreshInterval, Theme: AvailableThemes[m.currentThemeIdx].Name})
			return m, tea.Batch(doTick(m.refreshInterval, m.tickID), m.scanFleet())
		case "1":
			m.refreshInterval = 2
			m.autoRefresh = true
			m.tickID++
			return m, tea.Batch(doTick(m.refreshInterval, m.tickID), m.scanFleet())
		case "2":
			m.refreshInterval = 10
			m.autoRefresh = true
			m.tickID++
			return m, tea.Batch(doTick(m.refreshInterval, m.tickID), m.scanFleet())
		case "3":
			m.refreshInterval = 30
			m.autoRefresh = true
			m.tickID++
			return m, tea.Batch(doTick(m.refreshInterval, m.tickID), m.scanFleet())
		case "6":
			m.refreshInterval = 60
			m.autoRefresh = true
			m.tickID++
			return m, tea.Batch(doTick(m.refreshInterval, m.tickID), m.scanFleet())
		case "t":
			m.currentThemeIdx = (m.currentThemeIdx + 1) % len(AvailableThemes)
			th := AvailableThemes[m.currentThemeIdx]
			m.diskProgress = progress.New(progress.WithGradient(string(th.Success), string(th.Primary)), progress.WithWidth(28))
			m.ramProgress = progress.New(progress.WithGradient(string(th.Secondary), string(th.Primary)), progress.WithWidth(28))
			m.spinner.Style = lipgloss.NewStyle().Foreground(th.Primary)
			saveUserSettings(UserSettings{RefreshInterval: m.refreshInterval, Theme: th.Name})
			return m, nil
		case "r":
			m.autoRefresh = !m.autoRefresh
			m.tickID++
			if m.autoRefresh {
				return m, tea.Batch(doTick(m.refreshInterval, m.tickID), m.scanFleet())
			}
			return m, nil
		case "l", "L":
			m.liveDiag = !m.liveDiag
			if m.liveDiag && !m.cmdRunning && len(m.vms) > 0 && m.selectedVM < len(m.vms) {
				vm := m.vms[m.selectedVM]
				if vm.Status != "SHUT OFF" {
					m.cmdRunning = true
					return m, tea.Batch(
						m.spinner.Tick,
						runCommand(vm, m.selectedVM, m.commands[m.selectedCmd]),
					)
				}
			}
			return m, nil
		case "enter":
			if len(m.vms) == 0 {
				m.viewport.SetContent("// No hosts in inventory.\n// Press [n] or [+] to add your first Linux host.")
				return m, nil
			}
			vm := m.vms[m.selectedVM]
			if vm.Status == "SHUT OFF" {
				m.viewport.SetContent(fmt.Sprintf("// ERROR: Target host '%s' (%s) is SHUT OFF or unreachable.", vm.Name, vm.IP))
				return m, nil
			}
			m.cmdRunning = true
			return m, tea.Batch(
				m.spinner.Tick,
				runCommand(vm, m.selectedVM, m.commands[m.selectedCmd]),
			)
		case "a":
			return m, m.scanFleet()
		}

	case singleExecMsg:
		m.cmdRunning = false
		m.lastCmdRun = msg.refreshed
		if msg.vmIndex < len(m.vms) {
			vm := &m.vms[msg.vmIndex]
			if msg.err != nil {
				m.activeOutput = fmt.Sprintf("Error on %s (%s):\n%v", vm.Name, vm.IP, msg.err)
			} else {
				headerInfo := fmt.Sprintf("// Target: %s (%s) | Command: %s | Time: %v | Refreshed: %s\n",
					vm.Name, vm.IP, msg.cmdString, msg.duration, msg.refreshed.Format("15:04:05"))
				m.activeOutput = headerInfo + msg.stdout
			}
			m.viewport.SetContent(m.activeOutput)
		}
		return m, nil

	case fleetScanMsg:
		m.lastHeartbeat = msg.scanAt
		m.vms = msg.results
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m model) theme() UITheme {
	if m.currentThemeIdx < 0 || m.currentThemeIdx >= len(AvailableThemes) {
		return AvailableThemes[0]
	}
	return AvailableThemes[m.currentThemeIdx]
}

func (m model) View() string {
	th := m.theme()
	var b strings.Builder
	cyan := th.Primary
	muted := th.Muted
	panel := th.Background
	line := th.Line

	title := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render(" FLEET / CONTROL ROOM ")
	version := lipgloss.NewStyle().Foreground(muted).Render("  community resource monitor")
	b.WriteString(lipgloss.NewStyle().Background(th.HeaderBg).Padding(0, 1).Render(title + version))

	if m.autoRefresh {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(th.Success).Render("  ● LIVE"))
	} else {
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(th.Warning).Render("  ◌ PAUSED"))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf("  heartbeat %s", m.lastHeartbeat.Format("15:04:05"))))
	b.WriteString("  " + lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(th.Secondary).Padding(0, 1).Render(fmt.Sprintf("[t] Theme: %s", th.Name)))
	b.WriteString("\n\n")

	if m.addModalOpen {
		modalBox := lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(cyan).Background(th.HeaderBg).Padding(1, 3).Width(76)
		var content strings.Builder
		content.WriteString(lipgloss.NewStyle().Bold(true).Foreground(cyan).Render("ADD LINUX HOST") + "\n\n")
		labels := []string{"Host name", "IP address / FQDN", "SSH username"}
		for i := range m.inputs {
			label := lipgloss.NewStyle().Foreground(muted).Width(20)
			if i == m.addFocusIdx {
				label = lipgloss.NewStyle().Bold(true).Foreground(cyan).Width(20)
			}
			content.WriteString(label.Render(labels[i]) + m.inputs[i].View() + "\n")
		}
		if m.modalError != "" {
			content.WriteString("\n" + lipgloss.NewStyle().Bold(true).Foreground(th.Danger).Render(m.modalError) + "\n")
		}
		content.WriteString("\n" + lipgloss.NewStyle().Foreground(muted).Render("Tab next  •  Enter save  •  Esc cancel"))
		b.WriteString(modalBox.Render(content.String()) + "\n")
		return b.String()
	}

	online, offline := 0, 0
	for _, vm := range m.vms {
		if vm.Status == "ONLINE" {
			online++
		} else if vm.Status == "SHUT OFF" {
			offline++
		}
	}
	stat := func(label, value string, color lipgloss.Color, width int) string {
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(line).Background(panel).Padding(0, 1).Width(width).Render(
			lipgloss.NewStyle().Foreground(muted).Render(label) + "\n" + lipgloss.NewStyle().Bold(true).Foreground(color).Render(value))
	}

	var refreshButtons strings.Builder
	for _, iv := range []int{2, 10, 30, 60} {
		if iv == m.refreshInterval && m.autoRefresh {
			refreshButtons.WriteString(lipgloss.NewStyle().Bold(true).Foreground(th.Background).Background(th.Secondary).Render(fmt.Sprintf(" %ds ", iv)))
		} else if iv == m.refreshInterval && !m.autoRefresh {
			refreshButtons.WriteString(lipgloss.NewStyle().Bold(true).Foreground(th.Background).Background(th.Warning).Render(fmt.Sprintf(" %ds ", iv)))
		} else {
			refreshButtons.WriteString(lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf(" %ds", iv)))
		}
	}
	cardLabel := "REFRESH [i]"
	if !m.autoRefresh {
		cardLabel = "REFRESH [r] PAUSED"
	}

	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
		stat("HOSTS", fmt.Sprintf("%d", len(m.vms)), cyan, 18),
		stat("ONLINE", fmt.Sprintf("%d", online), th.Success, 18),
		stat("OFFLINE", fmt.Sprintf("%d", offline), th.Danger, 18),
		stat(cardLabel, refreshButtons.String(), th.Secondary, 25)) + "\n")

	section := lipgloss.NewStyle().Bold(true).Foreground(th.Text)
	matchedHosts := m.filteredVMIndices()
	activeQuery := strings.TrimSpace(m.searchInput.Value())

	const maxVisibleHosts = 6
	startIdx := 0
	endIdx := len(matchedHosts)
	if len(matchedHosts) > maxVisibleHosts {
		selPos := 0
		for i, idx := range matchedHosts {
			if idx == m.selectedVM {
				selPos = i
				break
			}
		}
		startIdx = selPos - maxVisibleHosts/2
		if startIdx < 0 {
			startIdx = 0
		}
		if startIdx+maxVisibleHosts > len(matchedHosts) {
			startIdx = len(matchedHosts) - maxVisibleHosts
		}
		endIdx = startIdx + maxVisibleHosts
	}

	if m.searchActive {
		countBadge := lipgloss.NewStyle().Foreground(th.Success).Render(fmt.Sprintf("(%d matches)", len(matchedHosts)))
		if len(matchedHosts) == 0 {
			countBadge = lipgloss.NewStyle().Foreground(th.Danger).Render("(0 matches)")
		} else if len(matchedHosts) > maxVisibleHosts {
			countBadge = lipgloss.NewStyle().Foreground(th.Success).Render(fmt.Sprintf("(%d-%d of %d)", startIdx+1, endIdx, len(matchedHosts)))
		}
		prompt := lipgloss.NewStyle().Bold(true).Foreground(cyan).Render("SEARCH: ")
		shortcuts := lipgloss.NewStyle().Foreground(muted).Render("  Enter apply  •  Esc close")
		boxContent := prompt + m.searchInput.View() + "  " + countBadge + "   " + shortcuts
		searchBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cyan).
			Background(th.HeaderBg).
			Width(92).
			Padding(0, 1).
			Render(boxContent)
		b.WriteString(searchBox + "\n")
	} else if activeQuery != "" {
		filterTag := fmt.Sprintf("  filter: %q (%d matches)  [Esc] clear  [/] edit", activeQuery, len(matchedHosts))
		if len(matchedHosts) > maxVisibleHosts {
			filterTag = fmt.Sprintf("  filter: %q (showing %d-%d of %d)  [Esc] clear  [/] edit", activeQuery, startIdx+1, endIdx, len(matchedHosts))
		}
		b.WriteString(section.Render("HOST INVENTORY") + lipgloss.NewStyle().Foreground(cyan).Render(filterTag) + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(line).Render(strings.Repeat("─", 94)) + "\n")
	} else {
		paging := ""
		if len(matchedHosts) > maxVisibleHosts {
			paging = fmt.Sprintf(" (%d-%d of %d)", startIdx+1, endIdx, len(matchedHosts))
		}
		b.WriteString(section.Render("HOST INVENTORY") + lipgloss.NewStyle().Foreground(muted).Render("  ↑/↓ select"+paging+"  [/] search") + "\n")
		b.WriteString(lipgloss.NewStyle().Foreground(line).Render(strings.Repeat("─", 94)) + "\n")
	}

	if len(m.vms) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(th.Warning).Render("  No hosts configured. Press [n] to add one.\n"))
	} else if len(matchedHosts) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(th.Warning).Render(fmt.Sprintf("  ⚠️  No VMs match %q. Press Esc to clear search.\n", activeQuery)))
	} else {
		for _, i := range matchedHosts[startIdx:endIdx] {
			vm := m.vms[i]
			selected := i == m.selectedVM
			prefix := "  "
			rowBg := th.Background
			textColor := th.Text
			if selected {
				prefix = "▸ "
				rowBg = th.Highlight
				textColor = cyan
			}
			statusColor := th.Warning
			if vm.Status == "ONLINE" {
				statusColor = th.Success
			} else if vm.Status == "SHUT OFF" {
				statusColor = th.Danger
			} else if vm.Status == "DEMO" {
				statusColor = th.Secondary
			}
			metrics := fmt.Sprintf("disk %3.0f%%   ram %3.0f%%   load %-14s", vm.DiskPercent*100, vm.RAMPercent*100, vm.CPULoad)
			nameCol := lipgloss.NewStyle().Foreground(textColor).Background(rowBg).Width(20).Render(prefix + vm.Name)
			ipCol := lipgloss.NewStyle().Foreground(textColor).Background(rowBg).Width(18).Render(vm.IP)
			statusCol := lipgloss.NewStyle().Bold(true).Foreground(statusColor).Background(rowBg).Width(18).Render("● " + vm.Status)
			metricsCol := lipgloss.NewStyle().Foreground(muted).Background(rowBg).Width(38).Render(metrics)
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Left, nameCol, ipCol, statusCol, metricsCol) + "\n")
		}
	}

	if len(m.vms) > 0 && m.selectedVM < len(m.vms) {
		vm := m.vms[m.selectedVM]
		b.WriteString("\n" + section.Render("SELECTED HOST") + lipgloss.NewStyle().Foreground(muted).Render(fmt.Sprintf("  %s  •  %s", vm.Name, vm.IP)) + "\n")
		if vm.Status == "SHUT OFF" {
			b.WriteString(lipgloss.NewStyle().Foreground(th.Danger).Render("  Host is powered off or unreachable.\n"))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(muted).Render("  DISK ") + m.diskProgress.ViewAs(vm.DiskPercent) + fmt.Sprintf(" %s/%s\n", vm.DiskUsed, vm.DiskTotal))
			b.WriteString(lipgloss.NewStyle().Foreground(muted).Render("  RAM  ") + m.ramProgress.ViewAs(vm.RAMPercent) + fmt.Sprintf(" %s/%s\n", vm.RAMUsed, vm.RAMTotal))
		}
	}

	activeCmd := m.commands[m.selectedCmd]
	liveStatus := lipgloss.NewStyle().Foreground(muted).Render("○ STREAM OFF [l]")
	if m.liveDiag {
		liveStatus = lipgloss.NewStyle().Bold(true).Foreground(th.Success).Render(fmt.Sprintf("● LIVE STREAM (%ds) [l]", m.refreshInterval))
	}
	b.WriteString("\n" + section.Render("DIAGNOSTICS") + "  " + lipgloss.NewStyle().Foreground(th.Secondary).Bold(true).Render(activeCmd.Icon+" "+activeCmd.Label) + "  " + liveStatus + lipgloss.NewStyle().Foreground(muted).Render("  [d] choose  [Enter] run") + "\n")
	if m.dropdownOpen {
		menu := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(th.Secondary).Background(th.HeaderBg).Padding(0, 1)
		var items strings.Builder
		for i, cmd := range m.commands {
			style := lipgloss.NewStyle().Foreground(muted)
			prefix := "  "
			if i == m.dropdownIdx {
				prefix = "▸ "
				style = lipgloss.NewStyle().Bold(true).Foreground(cyan).Background(th.Highlight)
			}
			items.WriteString(style.Render(fmt.Sprintf("%s%s %-26s %s", prefix, cmd.Icon, cmd.Label, cmd.Description)) + "\n")
		}
		b.WriteString(menu.Render(items.String()) + "\n")
	} else {
		output := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(line).Padding(0, 1).Height(m.viewport.Height)
		b.WriteString(output.Render(m.viewport.View()) + "\n")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(muted).Render("\n[/] search  [i] interval (2s/10s/30s/60s)  [l] live stream  [r] pause  [n] add  [x] remove  [Tab] diag  [t] theme  [q] quit"))
	return b.String()
}

type DaemonServer struct {
	mu  sync.RWMutex
	vms []VM
}

func runDaemon(addr string) {
	fmt.Printf("\n📡 [fleet-daemon] Starting headless polling daemon on %s\n", addr)
	vms := loadInventory()
	fmt.Printf("📋 [fleet-daemon] Loaded %d host(s) from inventory. Polling every 2s via SSH (Zero TUI)...\n", len(vms))

	ds := &DaemonServer{vms: vms}

	// Initial sweep
	ds.vms = performScan(ds.vms)

	// Background polling loop
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			current := loadInventory()
			updated := performScan(current)
			ds.mu.Lock()
			ds.vms = updated
			ds.mu.Unlock()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ds.mu.RLock()
		defer ds.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"hosts":  len(ds.vms),
			"time":   time.Now().Format(time.RFC3339),
		})
	})
	mux.HandleFunc("/api/v1/telemetry", func(w http.ResponseWriter, r *http.Request) {
		ds.mu.RLock()
		defer ds.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ds.vms)
	})
	mux.HandleFunc("/api/v1/hosts", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var cfg HostConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		current := loadInventoryConfigs()
		for _, c := range current {
			if strings.EqualFold(c.Name, cfg.Name) {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		current = append(current, cfg)
		_ = saveInventory(current)
		ds.mu.Lock()
		ds.vms = performScan(loadInventory())
		ds.mu.Unlock()
		fmt.Printf("➕ [fleet-daemon] Added host '%s' (%s) requested by viewer client\n", cfg.Name, cfg.IP)
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/v1/hosts/delete", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "missing name parameter", http.StatusBadRequest)
			return
		}
		current := loadInventoryConfigs()
		var filtered []HostConfig
		for _, c := range current {
			if !strings.EqualFold(c.Name, name) {
				filtered = append(filtered, c)
			}
		}
		_ = saveInventory(filtered)
		ds.mu.Lock()
		ds.vms = performScan(loadInventory())
		ds.mu.Unlock()
		fmt.Printf("🗑️ [fleet-daemon] Deleted host '%s' requested by viewer client\n", name)
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\n🛑 [fleet-daemon] Shutting down daemon gracefully...")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		os.Exit(0)
	}()

	fmt.Printf("✅ [fleet-daemon] Ready. Clients can connect with: ./fleet-tui --connect %s\n\n", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "❌ [fleet-daemon] Error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	daemonAddr := flag.String("daemon", "", "Run in headless background daemon mode on address (e.g. :8080)")
	connectAddr := flag.String("connect", "", "Connect TUI viewer to a running fleet-daemon (e.g. localhost:8080)")
	flag.Parse()

	if *daemonAddr != "" {
		runDaemon(*daemonAddr)
		return
	}

	remoteURL := ""
	if *connectAddr != "" {
		addr := *connectAddr
		if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
			addr = "http://" + addr
		}
		remoteURL = addr
	}

	p := tea.NewProgram(initialModel(remoteURL), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
