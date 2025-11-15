# FloraLab

![FloraLab logo](./logo/logo.png)

FloraLab is a small multi-language project combining a Python CLI for managing Flower-AI federated learning deployments on SLURM clusters and Go-based tooling under `florago/`.

This repository contains utilities to initialize, run, and stop Flower-AI stacks on SLURM, plus helper tooling for packaging and deployment.

## Key features

- Automated deployment of Flower-AI stacks on SLURM
- Integration between the Python CLI (`FloraLab`) and Go utilities (`florago`)
- SSH tunnel management for secure communication
- Simple workflow commands: `init`, `run`, `stop`

## Prerequisites

- macOS or Linux (instructions below use zsh)
- Go 1.20+ (for `florago`)
- Python 3.8+ and pip
- SSH access to a SLURM cluster (login node credentials)

## Quick start

### 1) Build or run Go tooling (optional)

If you need the Go binaries or the `florago` helper utilities:

```bash
cd florago
# Use the Makefile if present
make build
# or build manually
go build -o ./bin/florago ./...
```

You can also run specific commands directly with `go run`, for example:

```bash
cd florago
go run ./cmd/flowerserver.go
```

### 2) Python environment & install

From the repository root:

```bash
# create and activate a venv (zsh)
python3 -m venv .venv
source .venv/bin/activate

# install the package in editable mode
pip install -e .
```

### 3) Use the CLI

Examples (from repo root):

```bash
# initialize config in pyproject.toml
floralab-cli init your-slurm-login-node.edu

# run a federated learning job using 3 client nodes
floralab-cli run --nodes 3

# stop a running stack
floralab-cli stop
```

See `FloraLab/cli.py` for more CLI options and usage details.

## Repository layout

- `pyproject.toml` — Python packaging config.
- `florago/` — Go code, Makefile, and utilities.
	- `cmd/` — Go command entrypoints (e.g., `flowerserver.go`, `flowerclient.go`).
	- `utils/` — Go helpers used across the Go toolchain.
- `FloraLab/` — Python package and CLI (`cli.py`).
- `logo/` — Project logo (SVG) referenced in this README.

## Configuration

The CLI stores minimal configuration in `pyproject.toml` under a `[tool.floralab]` table when you run `floralab-cli init` (see `FloraLab/cli.py` for exact keys).

Example (added by `init`):

```toml
[tool.floralab]
login-node = "your-slurm-login-node.edu"

[tool.flwr.federations.floralab]
address = "127.0.0.1:9093"
insecure = true
root-certificates = null
```

## Development workflows

- Run Python tests (from repo root, after activating venv):

```bash
pytest
```

- Format/lint: use `black`/`ruff` (Python) and `gofmt`/`golangci-lint` (Go).

## Contributing

1. Fork the repo and create a branch.
2. Run tests and linters locally.
3. Open a PR describing the change.

Please include tests for new features and follow the repository style.

## License

This project is proprietary. A `LICENSE` file is included in the repository root that describes the terms under which the software may be used.

Short summary:

- Copyright (c) 2025 `trinh-tnp` — All Rights Reserved.
- No rights to use, copy, modify, distribute, sublicense, or create derivative works are granted except under a separate, written license agreement with the copyright holder.

Please see the full text in `LICENSE` for details and contact information.

## Contact

Maintainer: `trinh-tnp` (repo owner)

---

If you'd like, I can also:

- Expand the CLI command docs with actual flags parsed by `FloraLab/cli.py`.
- Add a `LICENSE` file (please tell me which license to use).
- Add a CI workflow that builds the Go tools and runs Python tests on push.
