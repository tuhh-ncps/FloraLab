# FloraLab Documentation

Complete guide for FloraLab - A federated learning platform with florago backend (Go) and floralab frontend (Python) for SLURM clusters.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Installation](#installation)
4. [Quick Start](#quick-start)
5. [florago Backend](#florago-backend)
6. [floralab CLI](#floralab-cli)
7. [SLURM Integration](#slurm-integration)
8. [Remote Debugging](#remote-debugging)
9. [Web Dashboard](#web-dashboard)
10. [Best Practices](#best-practices)
11. [Troubleshooting](#troubleshooting)

---

## Overview

FloraLab is a complete federated learning infrastructure designed for SLURM HPC clusters. It consists of two main components:

- **florago**: Go backend providing REST API, Flower-AI orchestration, and SLURM job management
- **floralab**: Python CLI that orchestrates the complete federated learning workflow

### Key Features

✅ **Automated Deployment** - One-command deployment of Flower-AI stacks  
✅ **SLURM Integration** - Native support for HPC cluster job scheduling  
✅ **Reverse Proxy** - Caddy-based proxy for secure API access  
✅ **Remote Debugging** - Embedded Delve debugger for Go applications  
✅ **Web Dashboard** - Brutal minimalist UI with pastel colors for monitoring  
✅ **SSH Tunneling** - Secure communication between local machine and cluster  

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     LOCAL MACHINE                            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  floralab-cli (Python)                              │   │
│  │  - init, run, stop, ui commands                     │   │
│  │  - SSH tunnel management                            │   │
│  │  - Web dashboard (FastAPI + Alpine.js)             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          │ SSH Tunnel
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                   SLURM LOGIN NODE                           │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  florago REST API (Go)                              │   │
│  │  - /health, /api/monitoring, /api/spin              │   │
│  │  - FlowerStackManager (state coordination)         │   │
│  │  - SLURM job script generation                      │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │  Caddy Reverse Proxy                                │   │
│  │  - Control API: 0.0.0.0:9093 -> server:9093        │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          │ SLURM Job
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                  SLURM COMPUTE NODES                         │
│  ┌──────────────────────────┐  ┌──────────────────────┐    │
│  │  Node 1 (Server)         │  │  Node 2-N (Clients)  │    │
│  │  - flower-superlink      │  │  - flower-supernode  │    │
│  │  - flower-superexec      │  │  - flower-superexec  │    │
│  │    (serverapp)           │  │    (clientapp)       │    │
│  └──────────────────────────┘  └──────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
```

### Directory Structure

```
$HOME/.florago/                  # florago backend data (SLURM-safe)
├── bin/
│   ├── florago                  # florago binary
│   ├── caddy                    # Caddy reverse proxy
│   └── dlv                      # Delve debugger
├── tmp/                         # Temporary files (not /tmp!)
├── data/                        # Application data
├── logs/                        # Application logs
├── config/
│   └── Caddyfile               # Caddy configuration
└── venv/
    └── flowerai-env/           # Python virtual environment
        └── bin/
            ├── flower-superlink
            ├── flower-supernode
            └── flower-superexec

$HOME/my-fl-project/             # Your project (any location)
├── pyproject.toml              # Flower federation config
├── src/
│   ├── task.py                 # Flower task implementation
│   └── client_app.py           # Client application
└── server_app.py               # Server application
```

---

## Installation

### Prerequisites

**Local Machine:**
- Python 3.12+
- SSH access to SLURM cluster

**SLURM Cluster (Login Node):**
- Go 1.21+ (for building Caddy and Delve)
- Python 3.11+
- SLURM workload manager

### Install floralab

```bash
# Clone repository
git clone <repository-url>
cd FloraLab

# Build everything (florago + floralab)
make build-all

# Install floralab (includes florago binary)
pip install -e .

# Or use uv
uv pip install -e .
```

---

## Quick Start

### 1. Initialize Project

```bash
# Create Flower project with floralab config
floralab-cli init <slurm-login-node>

# This adds to pyproject.toml:
# [tool.floralab]
# login-node = "your-cluster.edu"
#
# [tool.flwr.federations.floralab]
# address = "127.0.0.1:9093"
# insecure = true
```

### 2. Run Federated Learning Job

```bash
# Start complete workflow (7 automated steps)
floralab-cli run --nodes 4 --partition gpu --time 01:00:00

# This will:
# 1. Copy florago binary to SLURM login node
# 2. Run florago init (install Caddy, Delve, Python env)
# 3. Start florago API server
# 4. Create SSH tunnel (localhost:8080 -> login-node:8080)
# 5. Submit SLURM job (POST /api/spin)
# 6. Wait for stack ready + update pyproject.toml
# 7. Execute: flwr run floralab .
```

### 3. Monitor with Web Dashboard

```bash
# Launch web dashboard
floralab-cli ui --port 3000

# Open browser: http://localhost:3000
```

### 4. Stop Job

```bash
# Cancel SLURM job and cleanup
floralab-cli stop
```

---

## florago Backend

### Commands

#### `florago init`
Initialize florago environment (Python, venv, Caddy, Delve)

```bash
florago init
```

Installs:
- Python 3.11+ verification
- Virtual environment: `$HOME/.florago/venv/flowerai-env`
- Flower-AI packages: `flwr[simulation]>=1.23.0`, `ray>=2.31.0`
- Caddy reverse proxy (built from source)
- Delve debugger v1.21.2

#### `florago start`
Start REST API server

```bash
florago start --host 0.0.0.0 --port 8080
```

**Endpoints:**
- `GET /health` - Health check
- `GET /api/monitoring` - Comprehensive monitoring (Flower + SLURM)
- `POST /api/spin` - Spin up Flower stack
- `GET /api/spin` - Get current stack status
- `DELETE /api/spin` - Tear down stack (cancel SLURM job)

**Internal (used by nodes):**
- `POST /api/flower/server` - Server node registration
- `POST /api/flower/client` - Client node registration
- `GET /api/flower/server` - Get server info for clients

#### `florago flowerserver`
Start Flower server stack (superlink + superexec-serverapp)

```bash
# Called automatically by SLURM job script
florago flowerserver
```

Environment variables:
- `FLOWER_SERVER_APP_IO_API_PORT` (default: 9091)
- `FLOWER_FLEET_API_PORT` (default: 9092)
- `FLOWER_CONTROL_API_PORT` (default: 9093)
- `FLORAGO_API_SERVER` (required)

#### `florago flowerclient`
Start Flower client stack (supernode + superexec-clientapp)

```bash
# Called automatically by SLURM job script
florago flowerclient
```

Environment variables:
- `FLOWER_CLIENT_APP_IO_API_PORT` (default: 9094)
- `FLORAGO_API_SERVER` (required)

### Building florago

```bash
# Build for current platform
cd florago
make build

# Build for all platforms
make build-all

# Outputs:
# - bin/florago-amd64  (Linux/macOS Intel/Windows)
# - bin/florago-arm64  (Apple Silicon M1/M2/M3)
```

---

## floralab CLI

### Commands

#### `floralab-cli init`
Initialize floralab configuration

```bash
floralab-cli init <login-node> [--dir .]
```

Adds configuration to `pyproject.toml`:
- `[tool.floralab]` - Login node hostname
- `[tool.flwr.federations.floralab]` - Flower federation settings

#### `floralab-cli run`
Complete federated learning workflow

```bash
floralab-cli run [OPTIONS]

Options:
  --nodes, -n INTEGER     Number of client nodes (default: 2)
  --partition, -p TEXT    SLURM partition
  --memory, -m TEXT       Memory per node (e.g., 4G)
  --time, -t TEXT         Time limit (e.g., 01:00:00)
  --dir, -d PATH          Project directory (default: current)
  --ssh-port INTEGER      Local SSH tunnel port (default: 8080)
```

**7-Step Workflow:**
1. `scp` florago binary to login node
2. `ssh florago init` - Initialize environment
3. Start florago API server with `nohup`
4. Create SSH tunnel (background process)
5. `POST /api/spin` - Submit SLURM job
6. Wait for stack ready, get control port, update `pyproject.toml`
7. Execute `flwr run floralab .`

#### `floralab-cli stop`
Stop Flower stack

```bash
floralab-cli stop [--dir .] [--ssh-port 8080]
```

Cancels SLURM job via `DELETE /api/spin`.

#### `floralab-cli ui`
Launch web dashboard

```bash
floralab-cli ui [--api-url URL] [--port 3000] [--host 127.0.0.1]
```

Starts FastAPI server with Alpine.js dashboard showing:
- API health status
- Flower stack state (job ID, status, nodes)
- Server node details (IP, ports)
- Client nodes status
- SLURM cluster information
- Auto-refresh every 5 seconds

#### Utility Commands

```bash
# Direct API interaction
floralab-cli spin <num-nodes>      # Spin up stack
floralab-cli status                 # Get stack status
floralab-cli down                   # Tear down stack
floralab-cli monitoring             # Get monitoring data
floralab-cli health                 # Check API health
```

---

## SLURM Integration

### Best Practices on Login Nodes

#### ✅ Do This

```bash
# Work in persistent storage
cd $HOME && floralab-cli init cluster.edu
cd $SCRATCH && floralab-cli init cluster.edu

# Use florago's managed directories
# Everything goes in $HOME/.florago/

# Quick commands on login node
florago start
floralab-cli run
```

#### ❌ Avoid This

```bash
# Don't work in /tmp
cd /tmp && floralab-cli init   # BAD

# Don't run compute-intensive work on login node
# SLURM jobs handle the heavy lifting

# Don't expose debug ports publicly
# Always use SSH tunnels
```

### Directory Principles

✅ **Always uses `$HOME/.florago`** - SLURM-safe internal storage  
✅ **Never fills `/tmp`** - Avoids login node disk issues  
✅ **Respects current directory** - Your projects anywhere  
✅ **Works in any location** - Home, scratch, project dirs  

### SLURM Job Script (Generated)

```bash
#!/bin/bash
#SBATCH --job-name=flower-stack
#SBATCH --nodes=5
#SBATCH --ntasks-per-node=1
#SBATCH --partition=gpu
#SBATCH --mem=4G
#SBATCH --time=01:00:00
#SBATCH --output=$HOME/.florago/logs/flower-stack-%j.out
#SBATCH --error=$HOME/.florago/logs/flower-stack-%j.err

export FLORAGO_API_SERVER=http://login-node:8080

FLORAGO_BIN=$HOME/.florago/bin/florago

# Start Caddy reverse proxy on server node
CADDY_BIN=$HOME/.florago/bin/caddy
CADDYFILE=$HOME/.florago/config/Caddyfile
if [ -f "$CADDY_BIN" ] && [ -f "$CADDYFILE" ]; then
  srun --nodes=1 --ntasks=1 --nodelist=$(scontrol show hostname $SLURM_JOB_NODELIST | head -n 1) \
    $CADDY_BIN run --config $CADDYFILE &
  sleep 2
fi

# Launch server on first node
srun --nodes=1 --ntasks=1 --nodelist=$(scontrol show hostname $SLURM_JOB_NODELIST | head -n 1) \
  $FLORAGO_BIN flowerserver &

# Launch clients on remaining nodes
if [ $SLURM_NNODES -gt 1 ]; then
  CLIENT_NODES=$(scontrol show hostname $SLURM_JOB_NODELIST | tail -n +2)
  for node in $CLIENT_NODES; do
    srun --nodes=1 --ntasks=1 --nodelist=$node \
      $FLORAGO_BIN flowerclient &
  done
fi

wait
```

### Resource Management

**Check quotas:**
```bash
quota -s
du -sh $HOME/.florago
```

**Clean up:**
```bash
rm -rf $HOME/.florago/tmp/*
find $HOME/.florago/logs -mtime +30 -delete
```

**Monitor usage:**
```bash
# Login node load
uptime
top

# Memory
free -h
ps aux | grep florago
```

---

## Remote Debugging

### Embedded Delve Solution

florago includes built-in Delve debugger installation and deployment.

#### Install Delve

```bash
# On SLURM login node
florago init   # Automatically installs Delve to $HOME/.florago/bin/dlv
```

Installs Delve v1.21.2 from source using:
```bash
go install github.com/go-delve/delve/cmd/dlv@v1.21.2
```

#### Debug florago Backend

**On SLURM login node:**
```bash
# Start debug server
dlv exec $HOME/.florago/bin/florago --headless --listen=:2345 --api-version=2 -- start
```

**On local machine:**
```bash
# Terminal 1: SSH tunnel
ssh -L 2345:localhost:2345 user@cluster

# Terminal 2: Connect debugger
dlv connect localhost:2345
```

#### VS Code Configuration

`.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Connect to Remote florago",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "remotePath": "${workspaceFolder}",
      "port": 2345,
      "host": "localhost",
      "showLog": true
    }
  ]
}
```

#### Common Delve Commands

```
(dlv) break main.main          # Set breakpoint
(dlv) break cmd/start.go:42    # Breakpoint at line
(dlv) continue                 # Continue execution
(dlv) next                     # Step over
(dlv) step                     # Step into
(dlv) print variable           # Print variable
(dlv) locals                   # Show local variables
(dlv) goroutines               # List goroutines
(dlv) stack                    # Show stack trace
```

### Security Considerations

1. **Always use SSH tunnels** - Never expose debugger port directly
2. **Firewall rules** - Block port 2345 from external access
3. **Authentication** - Delve has no built-in auth, rely on network security
4. **Production** - Never run debugger on production systems

---

## Web Dashboard

### Features

- **Brutal Minimalist Design** with pastel color palette
- **Real-time Monitoring** - Auto-refresh every 5 seconds
- **API Health Status** - Online/offline indicator
- **Flower Stack Details** - Job ID, status, nodes progress
- **Server Node Info** - IP, ports, status
- **Client Nodes Grid** - All client details at a glance
- **SLURM Cluster** - User, jobs, nodes information
- **Responsive Layout** - Works on desktop and tablet

### Design Style

Inspired by brutal minimalism with pastel accents:
- **Gradient background**: mint → lavender → cream
- **Pastel cards**: purple, green, blue, pink, yellow gradients
- **Black borders** with hard drop shadows (4px)
- **Bold typography**: Font-black for emphasis
- **Terminal blocks**: Dark background with green text

### Launch Dashboard

```bash
floralab-cli ui --port 3000
```

Open browser: `http://localhost:3000`

### API Proxy

Dashboard proxies requests to florago API:
- `GET /api/health` → `florago/health`
- `GET /api/monitoring` → `florago/api/monitoring`

---

## Best Practices

### 1. Project Organization

```bash
# Organize projects by type
$HOME/
├── .florago/              # florago internal (automatic)
├── fl-experiments/        # Federated learning projects
│   ├── mnist-cnn/
│   ├── shakespeare-lstm/
│   └── cifar10-resnet/
└── fl-production/         # Production deployments
```

### 2. Configuration Management

Use `pyproject.toml` for project settings:

```toml
[tool.floralab]
login-node = "cluster.edu"

[tool.flwr.federations.floralab]
address = "127.0.0.1:9093"
insecure = true

[tool.flwr.app]
num-supernodes = 10

[tool.flwr.app.config]
num-rounds = 5
local-epochs = 3
batch-size = 32
```

### 3. Resource Allocation

**Choose appropriate SLURM resources:**

```bash
# Small test (2 nodes, 10 minutes)
floralab-cli run --nodes 2 --time 00:10:00

# Medium experiment (10 nodes, 1 hour, GPU)
floralab-cli run --nodes 10 --partition gpu --time 01:00:00 --memory 8G

# Large deployment (100 nodes, 4 hours)
floralab-cli run --nodes 100 --time 04:00:00 --memory 16G
```

### 4. Monitoring

**Check job status:**
```bash
floralab-cli status          # Quick status
floralab-cli monitoring      # Detailed monitoring
floralab-cli ui              # Visual dashboard
```

**SLURM commands:**
```bash
squeue -u $USER              # Your jobs
scontrol show job <job-id>   # Job details
tail -f $HOME/.florago/logs/flower-stack-*.out
```

### 5. Cleanup

**After experiments:**
```bash
floralab-cli stop            # Stop current job
rm -rf $HOME/.florago/tmp/*  # Clean temp files
```

**Periodic maintenance:**
```bash
# Clean old logs (>30 days)
find $HOME/.florago/logs -mtime +30 -delete

# Check disk usage
du -sh $HOME/.florago
quota -s
```

---

## Troubleshooting

### Common Issues

#### 1. "florago binary not found"

**Symptom**: `floralab-cli run` fails with binary not found error.

**Solution**:
```bash
# Build florago
cd florago && make build-all

# Copy to floralab
cp bin/florago-amd64 ../floralab/bin/

# Rebuild floralab
pip install -e .
```

#### 2. "Connection refused" on SSH tunnel

**Symptom**: Dashboard can't connect to API.

**Check**:
```bash
# Verify tunnel is running
ps aux | grep "ssh -N -L"
netstat -an | grep 8080

# Restart tunnel manually
ssh -N -L 8080:localhost:8080 user@cluster &
```

#### 3. "Disk quota exceeded"

**Symptom**: florago init fails or logs stop writing.

**Solution**:
```bash
# Check usage
quota -s
du -sh $HOME/.florago/*

# Clean up
rm -rf $HOME/.florago/tmp/*
find $HOME/.florago/logs -mtime +7 -delete
```

#### 4. "SLURM job stuck in pending"

**Symptom**: Job never starts running.

**Check**:
```bash
squeue -u $USER
scontrol show job <job-id>

# Check why pending
scontrol show job <job-id> | grep Reason
```

**Common reasons**:
- Resources: Requesting too much memory/time
- Priority: Other jobs ahead in queue
- Partition: Wrong partition or partition down

#### 5. "Python version too old"

**Symptom**: florago init fails with Python version error.

**Solution**:
```bash
# Load newer Python module (if available)
module load python/3.11

# Or install locally
# Build Python from source in $HOME
```

#### 6. "Caddy failed to start"

**Symptom**: Reverse proxy not working.

**Check**:
```bash
# Verify Caddy is installed
ls -lh $HOME/.florago/bin/caddy

# Check Caddyfile syntax
$HOME/.florago/bin/caddy validate --config $HOME/.florago/config/Caddyfile

# Reinstall if needed
florago init  # Rebuilds Caddy
```

#### 7. "Flower stack never becomes ready"

**Symptom**: `floralab-cli run` times out waiting for stack.

**Debug**:
```bash
# Check SLURM job logs
tail -f $HOME/.florago/logs/flower-stack-*.out
tail -f $HOME/.florago/logs/flower-stack-*.err

# Check API monitoring
floralab-cli monitoring

# Manual SSH to compute node
squeue -u $USER  # Get node name
ssh <compute-node>
ps aux | grep flower
```

### Getting Help

**Check logs:**
```bash
# florago API logs
tail -100 ~/.florago/logs/florago-server.log

# SLURM job logs
ls -lt ~/.florago/logs/flower-stack-*.{out,err}

# Individual node logs (if available)
ssh <node> "cat /tmp/florago-*.log"
```

**Verify setup:**
```bash
# Check versions
florago version
python --version
sinfo --version

# Check environment
echo $HOME
echo $USER
df -h $HOME
quota -s
```

**Community Resources:**
- Flower-AI Documentation: https://flower.ai/docs/
- SLURM Documentation: https://slurm.schedmd.com/
- GitHub Issues: <repository-url>/issues

---

## Development

### Building from Source

```bash
# Clone repository
git clone <repository-url>
cd FloraLab

# Build everything
make build-all

# This builds:
# 1. florago binaries (amd64, arm64)
# 2. Copies binaries to floralab/bin/
# 3. Builds floralab wheel
```

### Project Structure

```
FloraLab/
├── florago/                    # Go backend
│   ├── cmd/                   # CLI commands
│   │   ├── root.go           # Root command, init
│   │   ├── start.go          # REST API server
│   │   ├── flowerserver.go   # Server node
│   │   └── flowerclient.go   # Client node
│   ├── utils/                 # Utilities
│   │   ├── caddy.go          # Caddy management
│   │   ├── debugger.go       # Delve installation
│   │   ├── flowerstack.go    # Stack state manager
│   │   ├── slurm.go          # SLURM commands
│   │   └── paths.go          # SLURM-safe paths
│   └── Makefile              # florago build
│
├── floralab/                  # Python CLI
│   ├── cli.py                # Main CLI (Typer)
│   ├── ui_server.py          # FastAPI server
│   ├── templates/
│   │   └── dashboard.html    # Web dashboard
│   └── bin/
│       └── florago-amd64     # Bundled binary
│
├── pyproject.toml            # floralab package config
├── Makefile                  # Parent build system
└── DOCUMENTATION.md          # This file
```

### Running Tests

```bash
# Go tests
cd florago
go test ./...

# Python tests (if available)
cd floralab
pytest
```

### Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open Pull Request

---

## License

MIT License - See LICENSE file for details

---

## Acknowledgments

- **Flower-AI**: Federated learning framework
- **Cobra**: Go CLI framework
- **Typer**: Python CLI framework
- **FastAPI**: Modern web framework
- **Alpine.js**: Reactive JavaScript
- **Tailwind CSS**: Utility-first CSS
- **Delve**: Go debugger
- **Caddy**: Modern web server

---

## Quick Reference

### florago Commands

```bash
florago init                   # Initialize environment
florago start                  # Start API server
florago flowerserver          # Start server node
florago flowerclient          # Start client node
florago version               # Show version
```

### floralab Commands

```bash
floralab-cli init <host>      # Initialize project
floralab-cli run              # Run FL job
floralab-cli stop             # Stop job
floralab-cli ui               # Launch dashboard
floralab-cli status           # Get status
floralab-cli monitoring       # Get monitoring data
floralab-cli health           # Check API health
```

### Environment Variables

```bash
HOME                          # User home (primary)
FLORAGO_API_URL              # API server URL
FLOWER_*_PORT                # Flower component ports
SLURM_JOB_ID                 # SLURM job ID
```

### Important Paths

```bash
$HOME/.florago/              # florago internal
$HOME/.florago/bin/          # Binaries (florago, caddy, dlv)
$HOME/.florago/logs/         # Logs
$HOME/.florago/config/       # Configs (Caddyfile)
```

---

**Last Updated**: November 2025  
**Version**: 0.1.0
