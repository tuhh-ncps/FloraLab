# FloraGo on SLURM Clusters

## Important: Directory Structure on SLURM

FloraGo is designed to work safely on SLURM login nodes and respects cluster filesystem conventions.

### Key Principles

✅ **Always uses `$HOME/.florago`** for internal data  
✅ **Never fills `/tmp`** - avoids login node issues  
✅ **Respects current working directory** for user projects  
✅ **Works in any location** - scratch, home, project directories  

### Directory Layout

```
$HOME/
└── .florago/              # FloraGo internal directory
    ├── bin/               # Delve debugger binary
    ├── tmp/               # Temporary files (not /tmp!)
    ├── data/              # Application data
    └── logs/              # Application logs

$HOME/my-project/          # Your project (any location)
├── config/
├── data/
└── logs/
```

## Installation on SLURM

### Method 1: Package Deployment (Recommended)

Build the package on your local machine:

```bash
# Local machine
florago debug package ./florago-deploy
tar -czf florago-slurm.tar.gz -C ./florago-deploy .
```

Deploy to SLURM cluster:

```bash
# Login node
cd $HOME
mkdir -p florago-tools
cd florago-tools
scp user@local:florago-slurm.tar.gz .
tar -xzf florago-slurm.tar.gz

# Run setup
./setup-slurm.sh
```

### Method 2: Direct Installation

If Go is available on the cluster:

```bash
# Login node
cd $HOME
git clone <repo> florago
cd florago
go build -o florago .

# Install debug tools (optional)
./florago debug install
```

## Usage Best Practices

### Initialize Projects in Proper Locations

```bash
# In your home directory
cd $HOME
florago init my-analysis

# In scratch space (if available)
cd $SCRATCH
florago init experiment-001

# In project directory
cd /projects/$USER
florago init simulation
```

### Avoid Common Mistakes

❌ **Don't work in /tmp**
```bash
cd /tmp && florago init project  # Bad - ephemeral storage
```

✅ **Work in persistent storage**
```bash
cd $HOME && florago init project  # Good
cd $SCRATCH && florago init project  # Good (if persistent)
```

❌ **Don't fill up login node temp**
```bash
export TMPDIR=/tmp  # Bad on SLURM
```

✅ **Use FloraGo's temp directory**
```bash
# Handled automatically by florago
# Uses $HOME/.florago/tmp
```

## SLURM Integration Examples

### Monitor SLURM Cluster

```bash
# From login node
florago slurm status
florago slurm queue
florago slurm info
```

### Debug Batch Jobs

Create a debug job script:

```bash
#!/bin/bash
#SBATCH --job-name=florago-debug
#SBATCH --nodes=1
#SBATCH --time=01:00:00

# Load modules if needed
module load go

# Start debug server on compute node
cd $HOME/florago-tools
./debug.sh slurm status
```

Connect from login node:

```bash
# Find compute node
squeue -u $USER

# SSH tunnel through login node
ssh -L 2345:compute-node:2345 user@cluster-login

# Connect debugger
dlv connect localhost:2345
```

## File System Considerations

### Home Directory ($HOME)
- **Size**: Usually limited (50-100GB)
- **Performance**: Moderate
- **Persistence**: Permanent
- **Use for**: Binaries, configs, small data

### Scratch Space ($SCRATCH)
- **Size**: Large (1-10TB)
- **Performance**: High
- **Persistence**: May be purged (30-90 days)
- **Use for**: Large datasets, temporary results

### Project Space (/projects)
- **Size**: Large (shared quota)
- **Performance**: Moderate to high
- **Persistence**: Permanent
- **Use for**: Shared data, long-term storage

### Local Temp (/tmp)
- **Size**: Small (varies)
- **Performance**: Fast
- **Persistence**: Job lifetime only
- **FloraGo stance**: **Avoided**

## Environment Variables

FloraGo respects these environment variables:

```bash
HOME          # User home directory (primary)
USER          # Username
SCRATCH       # Scratch space (if available)
SLURM_JOB_ID  # Job ID (when in batch job)
```

## Troubleshooting on SLURM

### "Disk quota exceeded"

Check your home directory usage:
```bash
du -sh $HOME/.florago
quota -s
```

Clean up if needed:
```bash
rm -rf $HOME/.florago/tmp/*
```

### "Permission denied" on login node

Ensure executables have correct permissions:
```bash
chmod +x florago dlv
```

### Debug server won't start

Check if port is available:
```bash
netstat -tuln | grep 2345
```

Use different port:
```bash
DEBUG_PORT=3456 ./debug.sh
```

### Go version too old

Check Go version:
```bash
go version  # Need 1.21+
```

Load newer module:
```bash
module load go/1.21
```

Or use pre-built package (no Go needed).

## Resource Management

### CPU Usage on Login Nodes

Login nodes are shared - be considerate:

```bash
# Check load
uptime
top

# Limit Go build parallelism if needed
GOMAXPROCS=2 go build
```

### Memory Usage

Monitor memory:
```bash
free -h
ps aux | grep florago
```

### Storage Cleanup

Periodic cleanup:
```bash
# Clean temp files
rm -rf $HOME/.florago/tmp/*

# Clean old logs
find $HOME/.florago/logs -mtime +30 -delete
```

## Integration with Job Scheduler

### SLURM Job Script Example

```bash
#!/bin/bash
#SBATCH --job-name=florago-job
#SBATCH --output=%x-%j.out
#SBATCH --error=%x-%j.err
#SBATCH --nodes=1
#SBATCH --ntasks=1
#SBATCH --time=01:00:00

module load go

cd $HOME/my-project
$HOME/florago-tools/florago slurm status
```

### Array Jobs

```bash
#!/bin/bash
#SBATCH --array=1-10

TASK_ID=$SLURM_ARRAY_TASK_ID
florago process --task $TASK_ID
```

## Security Considerations

### SSH Tunneling

Always use SSH tunnels for debug connections:

```bash
# Two-hop tunnel (local -> login -> compute)
ssh -L 2345:compute-node:2345 user@cluster-login

# Or with ProxyJump
ssh -J user@cluster-login -L 2345:localhost:2345 compute-node
```

### File Permissions

Protect sensitive data:
```bash
chmod 700 $HOME/.florago
chmod 600 $HOME/.florago/config/*
```

### Network Security

Debug server only on localhost:
```bash
# Good - only local connections
./dlv exec --listen=localhost:2345

# Bad - exposed to network
./dlv exec --listen=0.0.0.0:2345
```

## Summary: SLURM-Safe Practices

✅ **Do:**
- Use `$HOME/.florago` for internal data
- Work in proper project directories
- Clean up after yourself
- Use SSH tunnels for debugging
- Monitor resource usage on login nodes

❌ **Don't:**
- Fill up `/tmp` on login nodes
- Run compute-intensive tasks on login nodes
- Leave debug servers running indefinitely
- Store large files in home directory
- Expose debug ports to network

FloraGo is designed with these principles in mind and handles them automatically! 🎯
