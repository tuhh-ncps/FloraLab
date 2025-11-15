# FloraGo Remote Debugging - Embedded Delve Solution

## Overview

FloraGo now includes built-in tools to install, package, and ship the Delve debugger alongside your application, solving the problem of debugging on remote machines where you cannot install Delve.

## Features

✅ **Auto-install Delve** - Builds Delve from source using `go install`  
✅ **Package Creator** - Bundles florago + dlv + helper scripts  
✅ **No Pre-requisites** - Only requires Go on the remote machine for initial install  
✅ **Helper Scripts** - Automated debug server startup  
✅ **Cross-platform** - Works on Linux, macOS, and Windows  

## Commands

### Check Debug Tools Status

```bash
florago debug status
```

Shows whether Delve is installed and where.

### Install Delve

```bash
florago debug install
```

Builds and installs Delve to `~/.florago/bin/dlv` using `go install`.

### Create Deployment Package

```bash
florago debug package <output-dir>
```

Creates a complete deployment package containing:
- `florago` - Your application binary
- `dlv` - Delve debugger binary
- `debug.sh` - Helper script to start debug server
- `DEBUG_README.md` - Usage instructions

### Start Debug Server

```bash
florago debug server [--port 2345] [args...]
```

Starts a debug server (auto-installs Delve if needed).

## Deployment Workflow

### Step 1: Create Package (Local Machine)

```bash
# Build florago
make build

# Create deployment package
./bin/florago debug package ./deploy

# Create tarball
tar -czf florago-debug.tar.gz -C ./deploy .
```

### Step 2: Transfer to Remote Machine

```bash
# Copy to remote
scp florago-debug.tar.gz user@remote:/path/to/destination

# On remote machine
tar -xzf florago-debug.tar.gz
```

### Step 3: Start Debug Server (Remote Machine)

```bash
# Start with helper script
./debug.sh

# Or start with arguments
./debug.sh slurm status

# Or use custom port
DEBUG_PORT=3456 ./debug.sh
```

### Step 4: Connect from Local Machine

```bash
# Terminal 1: Create SSH tunnel
ssh -L 2345:localhost:2345 user@remote-host

# Terminal 2: Connect debugger
dlv connect localhost:2345
```

## VS Code Remote Debugging

With the SSH tunnel active, use the provided launch configuration:

```json
{
  "name": "Connect to Remote Delve",
  "type": "go",
  "request": "attach",
  "mode": "remote",
  "remotePath": "${workspaceFolder}",
  "port": 2345,
  "host": "localhost"
}
```

Press F5 and select "Connect to Remote Delve".

## Package Contents

When you run `florago debug package`, it creates:

```
deploy/
├── florago          # Your application (3.9M)
├── dlv              # Delve debugger (17M)
├── debug.sh         # Helper script (executable)
└── DEBUG_README.md  # Usage instructions
```

Total size: ~21MB (compressed: ~7MB)

## How It Works

1. **Installation**: Uses `go install github.com/go-delve/delve/cmd/dlv@v1.21.2`
2. **Custom GOBIN**: Installs to `~/.florago/bin/` instead of system GOPATH
3. **Packaging**: Copies both florago and dlv binaries together
4. **Helper Script**: Simplifies starting debug server with correct flags

## Advantages Over Manual Installation

| Aspect | Manual Install | FloraGo Embedded |
|--------|---------------|------------------|
| **Prerequisites** | sudo access, package manager | Just Go (for build) |
| **Portability** | System-dependent | Self-contained |
| **Version Control** | System version | Bundled specific version |
| **Deployment** | Multi-step | Single tarball |
| **Consistency** | Varies by system | Identical everywhere |

## Alternative: On-demand Installation

If Go is available on the remote machine but you don't want to package:

```bash
# Remote machine
./florago debug install    # One-time setup
./florago debug server     # Starts debug server
```

This builds Delve from source on the remote machine (takes 1-2 minutes).

## Security Considerations

1. **SSH Tunneling**: Always use SSH tunnels, never expose debug port directly
2. **Firewall**: Block port 2345 from external access
3. **Production**: Never run debugger on production systems
4. **Cleanup**: Remove debug packages after troubleshooting

## Troubleshooting

### "Go is not installed"
- Install Go on remote machine
- Or use pre-packaged deployment (dlv already built)

### "Permission denied"
```bash
chmod +x debug.sh dlv florago
```

### "Connection refused"
- Verify SSH tunnel: `netstat -an | grep 2345`
- Check if debug server is running on remote
- Try different port: `DEBUG_PORT=3456 ./debug.sh`

### Symbols not loading in VS Code
- Ensure source code paths match
- Check `remotePath` in launch.json
- Verify same source version

## Example: Complete Debug Session

```bash
# === LOCAL MACHINE ===
# 1. Create package
cd florago
./bin/florago debug package /tmp/deploy
tar -czf florago-debug.tar.gz -C /tmp/deploy .

# 2. Transfer
scp florago-debug.tar.gz user@cluster:/home/user/

# === REMOTE MACHINE (SSH) ===
# 3. Extract and start
cd /home/user
tar -xzf florago-debug.tar.gz
./debug.sh slurm status

# === LOCAL MACHINE ===
# 4. New terminal - SSH tunnel
ssh -L 2345:localhost:2345 user@cluster

# 5. New terminal - Connect
dlv connect localhost:2345

# Or use VS Code F5 → "Connect to Remote Delve"
```

## Makefile Integration

The Makefile also includes debug targets:

```bash
make debug-build     # Build with debug symbols
make debug-server    # Start local debug server
make debug-remote    # Show remote debugging help
```

## Files Modified

- `utils/debugger.go` - Delve installation and management
- `cmd/debug.go` - CLI commands for debugging
- `.vscode/launch.json` - VS Code debug configurations
- `Makefile` - Build targets for debugging
- `DEBUGGING.md` - Comprehensive debugging guide

## Summary

FloraGo's embedded debugging solution eliminates the need to install Delve on remote machines. You can:

1. **Package once** - Create a self-contained debug bundle
2. **Deploy anywhere** - Single tarball with everything needed
3. **Debug remotely** - Full Delve functionality via SSH tunnel
4. **No dependencies** - Pre-built binaries travel together

This makes remote debugging on HPC clusters, Docker containers, or restricted environments straightforward and reliable.
