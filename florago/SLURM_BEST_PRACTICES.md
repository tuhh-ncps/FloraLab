# FloraGo on SLURM Clusters

## Best Practices for Login Nodes

When running FloraGo on SLURM cluster login nodes, follow these guidelines to avoid performance issues and quota problems:

### 1. Directory Structure

**Always work within `$HOME` or designated workspace:**

```bash
# Good practices
cd $HOME/florago-projects
florago init my-project

# Or use workspace if available
cd /workspace/$USER/florago-projects
florago init my-project
```

**Avoid these locations on login nodes:**
- `/tmp` - Often limited space, shared with all users
- `/var/tmp` - System temporary directory
- Root directory `/` - Never write here

### 2. FloraGo Directory Structure

FloraGo creates the following structure:

```
$HOME/.florago/          # Application data (created automatically)
├── bin/
│   └── dlv             # Delve debugger (~17MB)
└── cache/              # Future: cache data

$HOME/my-project/       # Your projects (created with 'florago init')
├── config/
│   └── florago.json
├── data/
├── logs/
└── README.md
```

### 3. Storage Locations

| Type | Location | Purpose |
|------|----------|---------|
| **Delve debugger** | `$HOME/.florago/bin/dlv` | One-time install, reused |
| **Project config** | `./config/` | Current project settings |
| **Project data** | `./data/` | Current project data |
| **Logs** | `./logs/` | Application logs |

### 4. SLURM-Specific Considerations

#### Login Node Etiquette

```bash
# ✅ Good: Quick commands, job submission
florago slurm queue
florago slurm status
florago init my-project

# ⚠️  Caution: Light debugging only
florago debug server &  # Only for quick tests

# ❌ Avoid: Heavy processing, long-running tasks
# Don't run compute-intensive work on login nodes
# Use sbatch/srun for actual workloads
```

#### Use Compute Nodes for Heavy Work

```bash
# Submit debug jobs to compute nodes
salloc -N 1 -t 1:00:00
# Now on compute node
cd $HOME/my-project
florago debug server --port 2345

# From local machine
ssh -L 2345:compute-node:2345 user@login-node
```

### 5. Quota Management

Check your disk usage regularly:

```bash
# Check home directory quota
quota -s

# Check FloraGo usage
du -sh $HOME/.florago
du -sh $HOME/florago-projects/*
```

Clean up when needed:

```bash
# Remove old debug packages
rm -rf $HOME/.florago/old-versions/

# Clean project logs
rm -rf $HOME/my-project/logs/*.log

# Remove Delve if not needed
rm -rf $HOME/.florago/bin/dlv
```

### 6. Environment Variables

Recommended environment variables for SLURM:

```bash
# In your ~/.bashrc or ~/.bash_profile

# Ensure HOME is used
export FLORAGO_HOME="$HOME/.florago"

# Set project directory
export FLORAGO_PROJECT_DIR="$HOME/florago-projects"

# Limit log size
export FLORAGO_MAX_LOG_SIZE="100M"
```

### 7. Network Considerations

**Debug server on compute nodes:**

```bash
# Request specific compute node
salloc -N 1 -t 2:00:00 --nodelist=node123

# Start debug server on compute node
ssh node123 "cd $HOME/my-project && ./florago debug server"

# Create tunnel from local machine
ssh -L 2345:node123:2345 user@login-node
```

**Port selection:**
- Default: 2345
- Avoid: 22 (SSH), 80/443 (HTTP/HTTPS), 3389 (RDP)
- Use high ports: 8000-9999 or 10000+

### 8. Job Submission Example

Create a job script that uses FloraGo:

```bash
#!/bin/bash
#SBATCH --job-name=florago-analysis
#SBATCH --output=$HOME/florago-projects/logs/job-%j.out
#SBATCH --error=$HOME/florago-projects/logs/job-%j.err
#SBATCH --time=01:00:00
#SBATCH --nodes=1
#SBATCH --ntasks=1

# Load modules if needed
module load go

# Change to project directory in $HOME
cd $HOME/florago-projects/my-project

# Run FloraGo commands
florago slurm status
florago slurm queue -u $USER

# Your analysis here
echo "Job completed"
```

Submit with:

```bash
sbatch job-script.sh
```

### 9. Multi-User Considerations

Each user has their own FloraGo installation:

```bash
user1: $HOME/.florago/    # User 1's installation
user2: $HOME/.florago/    # User 2's installation
```

Benefits:
- No permission issues
- Independent versions
- Quota isolation

### 10. Backup Recommendations

**Regular backups:**

```bash
# Backup configuration
tar -czf florago-config-$(date +%Y%m%d).tar.gz \
    $HOME/florago-projects/*/config/

# Backup to external storage (if available)
rsync -avz $HOME/florago-projects/ \
    /external/storage/$USER/florago-backup/
```

### 11. Troubleshooting on SLURM

#### "No space left on device"

```bash
# Check quota
quota -s

# Clean up
rm -rf $HOME/.florago/bin/dlv  # Remove if not debugging
find $HOME/florago-projects -name "*.log" -mtime +7 -delete
```

#### "Permission denied" on shared filesystems

```bash
# Ensure you're in your home directory
cd $HOME

# Check permissions
ls -la .florago/

# Reset if needed
chmod 700 $HOME/.florago
chmod 755 $HOME/.florago/bin/dlv
```

#### Slow filesystem performance

```bash
# Use local scratch if available
export TMPDIR=/local/scratch/$USER
mkdir -p $TMPDIR

# Copy data to local scratch for processing
cp $HOME/my-project/data/* $TMPDIR/
cd $TMPDIR
# ... process ...
# Copy results back
cp results/* $HOME/my-project/data/
```

### 12. Security on Shared Systems

**Protect your data:**

```bash
# Secure permissions
chmod 700 $HOME/.florago
chmod 700 $HOME/florago-projects

# Don't share debug packages
# They contain your application binary
```

**Debug server security:**

```bash
# Always use SSH tunnels, never expose ports directly
# Bad: florago debug server --listen=0.0.0.0:2345
# Good: florago debug server --listen=localhost:2345
```

### Summary Checklist

- [ ] Work in `$HOME` or designated workspace
- [ ] Check disk quota regularly
- [ ] Use compute nodes for heavy work
- [ ] Close debug servers when done
- [ ] Use SSH tunnels for remote debugging
- [ ] Set appropriate file permissions (700/600)
- [ ] Clean up old logs and temporary files
- [ ] Submit long-running tasks via SLURM
- [ ] Never run compute-intensive work on login nodes
- [ ] Backup important configurations

### Quick Reference

```bash
# Check where FloraGo stores data
florago debug status

# Initialize project in current directory (should be under $HOME)
cd $HOME/my-projects && florago init new-project

# Check disk usage
du -sh $HOME/.florago $HOME/florago-projects

# Clean up
rm -rf $HOME/.florago/bin/dlv        # Remove Delve
find $HOME -name "*.log" -mtime +30  # Find old logs
```

For more information, see:
- `REMOTE_DEBUGGING.md` - Remote debugging guide
- `DEBUGGING.md` - General debugging documentation
- `README.md` - Main FloraGo documentation
