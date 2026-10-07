# Fleet TUI 📡
> Lightweight, zero-dependency Terminal User Interface (TUI) for Linux fleet monitoring & diagnostics.

![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/License-MIT-green.svg)
![Architecture](https://img.shields.io/badge/Arch-Pure%20Go%20%7C%20Zero%20CGO-purple)

<p align="center">
  <img src="https://github.com/user-attachments/assets/18bfaf68-c117-4671-bd48-424479d7274a" alt="Fleet TUI Live Demo" width="900" />
</p>

**Fleet TUI** is a fast, keyboard-centric terminal dashboard designed to monitor and diagnose Linux servers and virtual machines in real time. Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

---

## ✨ Features

- **⚡ Zero Infrastructure**: Single static binary. No Docker, daemon, or database required.
- **🎨 7 Color Themes**:
  - **Dark Themes**: `Cyberpunk`, `Dracula`, `Nord`, `Matrix`
  - **Light Themes**: `Solarized Light`, `Paper Light`, `Catppuccin Latte`
  - Press **`[t]`** to cycle themes live with instant contrast adaptation.
- **🔍 Incremental Search**: Press **`[/]`** or **`[Ctrl+F]`** to filter hosts by name, IP, or status in real time.
- **📊 Real-Time Telemetry**: Multi-threaded SSH sweeps reporting Root Disk %, RAM %, and Kernel CPU load averages (1/5/15 min).
- **🛠 7 Inbuilt Diagnostics**: Run disk usage, memory/swap, top CPU processes, inode health, and network socket diagnostics with a single keypress. Toggle continuous live streaming with **`[l]`**.
- **➕ Easy Host Onboarding**: Add hosts interactively via **`[n]`** or maintain your inventory in standard JSON.

---

## 📡 Quick Start

### 1. Installation

#### Pre-built Binary
Download the latest binary from [Releases](https://github.com/csharmatech/fleet-tui/releases) or build directly with Go:

```bash
# Clone the repository
git clone https://github.com/csharmatech/fleet-tui.git
cd fleet-tui

# Build standalone binary (zero CGO required)
CGO_ENABLED=0 go build -ldflags="-s -w" -o fleet-tui .
```

### 2. Choose Your Deployment Mode

Fleet TUI supports two deployment modes out of the box using the same binary:

#### Mode A: Standalone Personal TUI (Default / Zero-Setup)
Best for individual developers, homelabs, or ad-hoc debugging on a single machine:
```bash
./fleet-tui
```
- Launches the interactive dashboard directly on your terminal.
- Performs local and direct SSH sweeps to your configured hosts.
- Zero listening network ports, zero daemons.

#### Mode B: Centralized Team Mode (Daemon + Shared Viewers)
Best for operations teams monitoring shared infrastructure without causing duplicate SSH connection storms across workstations.

##### 📋 Recommended Best Practices & Operational Workflow:

1. **Designate a Central Monitoring Server (Single Source of Truth)**:
   - Run the headless poller daemon 24/7 on your central management server or jump box:
     ```bash
     ./fleet-tui --daemon :8090 &
     ```
   - **Centralized Inventory Management**: Always perform fleet onboarding (`[n]` add) or node decommissioning (`[x]` delete) exclusively on this central server—either by editing `hosts.json` directly or by launching a local admin viewer on that server:
     ```bash
     ./fleet-tui --connect localhost:8090
     ```
   - This keeps your server inventory, authorized SSH keys, and network routes maintained in one authoritative, secure location.

2. **Configure Central Server Firewall**:
   > [!IMPORTANT]
   > **Firewall & Network Prerequisite**:
   > Ensure the listening TCP port (e.g. `8090/tcp`) is allowed through the firewall on your central daemon host so remote team viewers can connect:
   > ```bash
   > # RHEL / AlmaLinux / Rocky Linux (firewalld):
   > sudo firewall-cmd --permanent --add-port=8090/tcp && sudo firewall-cmd --reload
   >
   > # Ubuntu / Debian (ufw):
   > sudo ufw allow 8090/tcp
   > ```
   > *(Note: `fleet-tui --connect` includes an automatic fast-fail pre-flight probe that verifies port reachability before initializing the UI).*

3. **Connect Remote Team Viewers**:
   - Team members connect their visual TUI dashboards directly to the central server IP:
     ```bash
     ./fleet-tui --connect <central-server-ip>:8090
     ```
   - **Zero SSH Keys Required on Client Machines**: Remote operators do not need private SSH keys to target nodes on their laptops; the central daemon conducts all sweeps.
   - **Real-Time Fleet Synchronization**: Any host modifications made on the central server automatically sync and reflect across all connected viewer screens within 2 seconds.

---

### 3. Adding Hosts for Monitoring

You can add Linux hosts to your fleet using either the **Interactive In-App Modal** or by editing the **JSON Inventory File**.

#### Method A: Interactive In-App (Recommended)
1. Launch `fleet-tui`.
2. Press **`[n]`** or **`[+]`** to open the **Add Linux Host** modal.
3. Fill in the three fields:
   - **Host name**: A human-friendly alias (e.g., `web-prod-01`, `db-node-02`).
   - **IP address / FQDN**: The target IPv4 address or hostname (e.g., `192.168.1.50`).
   - **SSH username**: The Linux user on the **remote target machine** where your SSH key is authorized (e.g., `test`, `ubuntu`, `root`).
4. Press **`[Tab]`** or **`[↓]`** to move between fields.
5. Press **`[Enter]`** to save and begin monitoring immediately (or **`[Esc]`** to cancel).
6. To delete a host, select it in the list and press **`[x]`** or **`[Delete]`**.

#### Method B: Static JSON Inventory (`hosts.json`)
You can manage your fleet inventory declaratively in JSON. By default, `fleet-tui` checks:
1. `FLEET_HOSTS_PATH` *(environment variable override)*
2. `./hosts.json` *(current working directory)*
3. `~/.config/fleet-tui/hosts.json` *(user home directory configuration)*

Copy the example template or create your own:

```bash
cp hosts.json.example hosts.json
```

```json
[
  {
    "name": "localhost",
    "ip": "127.0.0.1",
    "user": "local",
    "is_local": true
  },
  {
    "name": "web-prod-01",
    "ip": "192.168.1.50",
    "user": "test"
  }
]
```

---

### 4. SSH Prerequisites for Remote Hosts

To monitor a remote host, `fleet-tui` connects via standard OpenSSH key-based authentication. Ensure the following prerequisites:

1. **Target User**: The `user` specified must match the account on the target machine that holds your authorized public key.
2. **Authorized Key**: Your monitoring machine's public key (`~/.ssh/id_rsa.pub` or derived from `~/.ssh/id_rsa`) must be present in the remote user's `~/.ssh/authorized_keys`.
   ```bash
   # One-line copy to remote machine
   ssh-copy-id -i ~/.ssh/id_rsa.pub user@<remote-ip>
   ```
3. **Verify Passwordless Access**: Test that passwordless execution works:
   ```bash
   ssh -o BatchMode=yes user@<remote-ip> "uptime"
   ```
   *(If this returns `Permission denied`, check that `~/.ssh` has `0700` permissions and `~/.ssh/authorized_keys` has `0600` permissions on the remote server).*
4. **POSIX Utilities**: Ensure `df`, `free`, and `uptime` are installed on the remote machine (standard in all Linux distributions).

---

## ⌨️ Keyboard Shortcuts

| Key | Action |
| :--- | :--- |
| **`[t]`** | Cycle through all 7 themes (`Cyberpunk` → `Dracula` → `Nord` → `Matrix` → `Solarized Light` → `Paper Light` → `Catppuccin Latte`) |
| **`[/]`**, **`[f]`** | Open live search bar |
| **`[Ctrl+U]`** | Clear search filter |
| **`[Esc]`** | Cancel search / close modal / close dropdown |
| **`[↑]` / `[k]`**, **`[↓]` / `[j]`** | Navigate host list |
| **`[d]`** / **`[c]`** | Open diagnostic commands dropdown menu |
| **`[Tab]`** | Cycle diagnostic command directly |
| **`[Enter]`** | Execute selected command on active VM |
| **`[n]`** / **`[+]`** | Add new Linux host |
| **`[x]`** / **`[Del]`** | Remove selected host |
| **`[l]`** | Toggle live diagnostic streaming (auto-executes command on heartbeat) |
| **`[r]`** | Pause / resume fleet telemetry auto-refresh |
| **`[i]`** | Cycle refresh interval (`2s` → `10s` → `30s` → `60s`) |
| **`[1]`, `[2]`, `[3]`, `[6]`** | Direct jump to `2s`, `10s`, `30s`, `60s` |
| **`[a]`** | Force immediate fleet rescan |
| **`[q]`** / **`[Ctrl+C]`** | Quit |

---

## 🛡 Security & Privacy

- **Zero Telemetry Collection**: No data ever leaves your local terminal.
- **Direct SSH Handshake**: All SSH connections use standard OpenSSH cryptographic keys.
- **Zero Daemon Overhead**: Runs on demand and exits cleanly.

---

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.
