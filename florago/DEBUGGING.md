# Remote Debugging Guide for FloraGo

This guide explains how to debug FloraGo on a remote machine using Delve.

## Prerequisites

Install Delve on both local and remote machines:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

## Method 1: Debug Server on Remote Machine

### On Remote Machine:

1. **Build with debug symbols:**
```bash
go build -gcflags="all=-N -l" -o florago .
```

2. **Start Delve server:**
```bash
# Listen on all interfaces (use with caution on public networks)
dlv exec ./florago --headless --listen=:2345 --api-version=2 --accept-multiclient

# Or listen only on localhost (safer, requires SSH tunnel)
dlv exec ./florago --headless --listen=localhost:2345 --api-version=2
```

3. **With arguments:**
```bash
dlv exec ./florago --headless --listen=:2345 --api-version=2 -- slurm status
```

### On Local Machine:

1. **SSH tunnel (if using localhost):**
```bash
ssh -L 2345:localhost:2345 user@remote-host
```

2. **Connect with Delve CLI:**
```bash
dlv connect localhost:2345
```

3. **Or use VS Code (see below)**

## Method 2: Attach to Running Process

### On Remote Machine:

1. **Start the program:**
```bash
./florago slurm status &
PID=$!
```

2. **Attach Delve:**
```bash
sudo dlv attach $PID --headless --listen=:2345 --api-version=2
```

### On Local Machine:

Connect as in Method 1.

## VS Code Remote Debugging

### 1. Create `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Connect to Remote Delve",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "remotePath": "/path/to/remote/florago",
      "port": 2345,
      "host": "localhost",
      "showLog": true,
      "trace": "verbose"
    }
  ]
}
```

### 2. Setup:

1. Ensure SSH tunnel is active: `ssh -L 2345:localhost:2345 user@remote-host`
2. Start Delve on remote machine (see Method 1)
3. In VS Code, press F5 or use "Run and Debug" panel
4. Select "Connect to Remote Delve"

## GoLand/IntelliJ Remote Debugging

### 1. Create Run Configuration:

1. Run → Edit Configurations
2. Add New Configuration → Go Remote
3. Set:
   - Host: `localhost`
   - Port: `2345`
   - On disconnect: Leave it running

### 2. Setup:

1. Create SSH tunnel
2. Start Delve on remote machine
3. Start debug configuration in GoLand

## Makefile Targets for Debugging

Add these to your `Makefile`:

```makefile
## debug-build: Build with debug symbols
debug-build:
	@echo "Building with debug symbols..."
	@go build -gcflags="all=-N -l" -o $(BUILD_DIR)/$(BINARY_NAME)-debug .

## debug-server: Start debug server (local)
debug-server: debug-build
	@echo "Starting debug server on :2345..."
	@dlv exec $(BUILD_DIR)/$(BINARY_NAME)-debug --headless --listen=:2345 --api-version=2

## debug-remote: Instructions for remote debugging
debug-remote:
	@echo "Remote Debugging Setup:"
	@echo ""
	@echo "1. On remote machine:"
	@echo "   make debug-build"
	@echo "   dlv exec ./bin/florago-debug --headless --listen=:2345 --api-version=2"
	@echo ""
	@echo "2. On local machine (in another terminal):"
	@echo "   ssh -L 2345:localhost:2345 user@remote-host"
	@echo ""
	@echo "3. Connect debugger:"
	@echo "   dlv connect localhost:2345"
	@echo "   OR use VS Code/GoLand remote debugging"
```

## Common Delve Commands

Once connected:

```
(dlv) break main.main          # Set breakpoint
(dlv) break cmd/root.go:42     # Breakpoint at line
(dlv) continue                 # Continue execution
(dlv) next                     # Step over
(dlv) step                     # Step into
(dlv) stepout                  # Step out
(dlv) print variable           # Print variable
(dlv) locals                   # Show local variables
(dlv) goroutines               # List goroutines
(dlv) stack                    # Show stack trace
(dlv) restart                  # Restart program
(dlv) exit                     # Exit debugger
```

## Security Considerations

1. **Use SSH tunnels** instead of exposing debugger port directly
2. **Firewall rules**: Block port 2345 from external access
3. **Authentication**: Delve doesn't have built-in auth, rely on network security
4. **Production**: Never run debugger on production systems

## Troubleshooting

### "could not attach to pid X"
- Use `sudo` when attaching to processes
- Check ptrace permissions: `echo 0 | sudo tee /proc/sys/kernel/yama/ptrace_scope`

### "connection refused"
- Verify Delve is running on remote
- Check SSH tunnel is active: `netstat -an | grep 2345`
- Verify firewall allows connection

### "API version mismatch"
- Ensure same Delve version on both machines
- Use `--api-version=2` flag consistently

### Symbols not loading
- Rebuild with: `-gcflags="all=-N -l"`
- Verify source paths match between local and remote
- Use `substitute-path` in Delve config

## Example Debugging Session

```bash
# Remote machine
cd /path/to/florago
make debug-build
dlv exec ./bin/florago-debug --headless --listen=:2345 --api-version=2 -- slurm status

# Local machine (terminal 1)
ssh -L 2345:localhost:2345 user@remote-host

# Local machine (terminal 2)
dlv connect localhost:2345

# In debugger
(dlv) break utils/slurm.go:45
(dlv) continue
(dlv) print client
(dlv) next
(dlv) locals
```

## Resources

- [Delve Documentation](https://github.com/go-delve/delve)
- [VS Code Go Debugging](https://github.com/golang/vscode-go/wiki/debugging)
- [Remote Debugging Guide](https://github.com/go-delve/delve/blob/master/Documentation/usage/dlv_exec.md)
