# FloraLab

FloraLab is a Python CLI tool for managing Flower-AI federated learning deployments on SLURM clusters.

## Features

- 🚀 Automated deployment of Flower-AI stacks on SLURM
- 🔧 Seamless integration with `florago` backend
- 📦 Bundled binaries - no manual setup required
- 🌐 SSH tunnel management for secure communication
- ⚡ Simple 3-command workflow: init, run, stop

## Installation

```bash
pip install floralab
```

Or with uv:

```bash
uv pip install floralab
```

## Quick Start

### 1. Initialize Configuration

```bash
floralab-cli init your-slurm-login-node.edu
```

This adds Flower-AI configuration to your `pyproject.toml`.

### 2. Run Federated Learning Job

```bash
floralab-cli run --nodes 3
```

This will:
- Copy florago to the SLURM cluster
- Initialize the environment
- Start the API server
- Spin up a Flower stack (1 server + 3 clients)
- Run your federated learning job with `flwr run`

### 3. Stop the Stack

```bash
floralab-cli stop
```

Tears down the Flower stack and cancels the SLURM job.

## Commands

### `floralab-cli init <login-node>`

Initialize floralab configuration in your project's `pyproject.toml`.

**Arguments:**
- `login-node` - Hostname of your SLURM login node

**Options:**
- `--dir, -d` - Project directory (default: current directory)

### `floralab-cli run`

Run a federated learning job on the SLURM cluster.

**Options:**
- `--nodes, -n` - Number of client nodes (default: 2)
- `--partition, -p` - SLURM partition
- `--memory, -m` - Memory per node (e.g., "4G")
- `--time, -t` - Time limit (e.g., "01:00:00")
- `--dir, -d` - Project directory (default: current directory)
- `--ssh-port` - Local port for SSH tunnel (default: 8080)

### `floralab-cli stop`

Stop the running Flower stack.

**Options:**
- `--dir, -d` - Project directory (default: current directory)
- `--ssh-port` - Local port for SSH tunnel (default: 8080)

## Configuration

FloraLab adds the following to your `pyproject.toml`:

```toml
[tool.floralab]
login-node = "your-slurm-login-node.edu"

[tool.flwr.federations.floralab]
address = "127.0.0.1:9093"
insecure = true
root-certificates = null
```

The `address` field is automatically updated when you run `floralab-cli run` to point to the actual Flower server.

## Requirements

- Python 3.12+
- SSH access to a SLURM cluster
- Flower AI installed (`flwr` command available)

## License

MIT
