package utils

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// SlurmClient provides utilities for monitoring SLURM clusters
type SlurmClient struct {
	logger *Logger
}

// NewSlurmClient creates a new SLURM client
func NewSlurmClient(logger *Logger) *SlurmClient {
	if logger == nil {
		logger = DefaultLogger
	}
	return &SlurmClient{logger: logger}
}

// CommandResult represents the result of a SLURM command execution
type CommandResult struct {
	Command string
	Output  string
	Error   error
}

// executeCommand runs a SLURM command and returns the output
func (s *SlurmClient) executeCommand(command string, args ...string) (*CommandResult, error) {
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()

	result := &CommandResult{
		Command: fmt.Sprintf("%s %s", command, strings.Join(args, " ")),
		Output:  string(output),
		Error:   err,
	}

	if err != nil {
		s.logger.Debug("Command failed: %s, Error: %v", result.Command, err)
		return result, fmt.Errorf("command execution failed: %w", err)
	}

	return result, nil
}

// Sinfo retrieves information about SLURM nodes and partitions
func (s *SlurmClient) Sinfo(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing sinfo command")
	return s.executeCommand("sinfo", args...)
}

// SinfoJSON retrieves sinfo output in JSON format
func (s *SlurmClient) SinfoJSON() (*CommandResult, error) {
	return s.Sinfo("--json")
}

// Scontrol shows the state of SLURM entities (nodes, jobs, partitions, etc.)
func (s *SlurmClient) Scontrol(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing scontrol command")
	return s.executeCommand("scontrol", args...)
}

// ScontrolShowNode shows detailed information about a specific node
func (s *SlurmClient) ScontrolShowNode(nodeName string) (*CommandResult, error) {
	return s.Scontrol("show", "node", nodeName)
}

// ScontrolShowJob shows detailed information about a specific job
func (s *SlurmClient) ScontrolShowJob(jobID string) (*CommandResult, error) {
	return s.Scontrol("show", "job", jobID)
}

// ScontrolShowPartition shows detailed information about a specific partition
func (s *SlurmClient) ScontrolShowPartition(partitionName string) (*CommandResult, error) {
	return s.Scontrol("show", "partition", partitionName)
}

// Squeue displays information about jobs in the queue
func (s *SlurmClient) Squeue(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing squeue command")
	return s.executeCommand("squeue", args...)
}

// SqueueJSON retrieves squeue output in JSON format
func (s *SlurmClient) SqueueJSON() (*CommandResult, error) {
	return s.Squeue("--json")
}

// SqueueUser displays jobs for a specific user
func (s *SlurmClient) SqueueUser(username string) (*CommandResult, error) {
	return s.Squeue("-u", username)
}

// Sstat displays status information about running jobs
func (s *SlurmClient) Sstat(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing sstat command")
	return s.executeCommand("sstat", args...)
}

// SstatJob displays statistics for a specific job
func (s *SlurmClient) SstatJob(jobID string) (*CommandResult, error) {
	return s.Sstat("-j", jobID, "--allsteps")
}

// Sacct displays accounting information for jobs
func (s *SlurmClient) Sacct(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing sacct command")
	return s.executeCommand("sacct", args...)
}

// SacctJSON retrieves sacct output in JSON format
func (s *SlurmClient) SacctJSON() (*CommandResult, error) {
	return s.Sacct("--json")
}

// SacctJob displays accounting information for a specific job
func (s *SlurmClient) SacctJob(jobID string) (*CommandResult, error) {
	return s.Sacct("-j", jobID, "--format=JobID,JobName,Partition,Account,AllocCPUS,State,ExitCode")
}

// SacctUser displays accounting information for a specific user
func (s *SlurmClient) SacctUser(username string, startTime string) (*CommandResult, error) {
	return s.Sacct("-u", username, "-S", startTime, "--format=JobID,JobName,State,Start,End,Elapsed")
}

// Sacctmgr manages SLURM accounting database
func (s *SlurmClient) Sacctmgr(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing sacctmgr command")
	return s.executeCommand("sacctmgr", args...)
}

// SacctmgrListUsers lists all users in the accounting database
func (s *SlurmClient) SacctmgrListUsers() (*CommandResult, error) {
	return s.Sacctmgr("list", "user", "-p")
}

// SacctmgrListAccounts lists all accounts in the accounting database
func (s *SlurmClient) SacctmgrListAccounts() (*CommandResult, error) {
	return s.Sacctmgr("list", "account", "-p")
}

// SacctmgrShowAssociation shows associations for a user or account
func (s *SlurmClient) SacctmgrShowAssociation(entity string) (*CommandResult, error) {
	return s.Sacctmgr("show", "association", entity, "-p")
}

// Sreport generates reports from accounting data
func (s *SlurmClient) Sreport(args ...string) (*CommandResult, error) {
	s.logger.Debug("Executing sreport command")
	return s.executeCommand("sreport", args...)
}

// SreportClusterUtilization generates cluster utilization report
func (s *SlurmClient) SreportClusterUtilization(startTime string) (*CommandResult, error) {
	return s.Sreport("cluster", "utilization", "-t", "percent", "start="+startTime)
}

// SreportUserTop generates top user report
func (s *SlurmClient) SreportUserTop(startTime string, topN int) (*CommandResult, error) {
	return s.Sreport("user", "top", "start="+startTime, fmt.Sprintf("topcount=%d", topN))
}

// SreportJobSizesByAccount generates job sizes by account report
func (s *SlurmClient) SreportJobSizesByAccount(startTime string) (*CommandResult, error) {
	return s.Sreport("job", "sizesbyccount", "start="+startTime)
}

// ClusterStatus represents the overall status of the SLURM cluster
type ClusterStatus struct {
	Nodes      map[string]int `json:"nodes"` // node states (idle, allocated, down, etc.)
	Jobs       map[string]int `json:"jobs"`  // job states (running, pending, etc.)
	TotalNodes int            `json:"total_nodes"`
	TotalJobs  int            `json:"total_jobs"`
}

// GetClusterStatus retrieves comprehensive cluster status
func (s *SlurmClient) GetClusterStatus() (*ClusterStatus, error) {
	s.logger.Info("Gathering cluster status...")

	status := &ClusterStatus{
		Nodes: make(map[string]int),
		Jobs:  make(map[string]int),
	}

	// Get node information
	sinfoResult, err := s.Sinfo("-h", "-o", "%T")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(sinfoResult.Output), "\n")
		for _, line := range lines {
			state := strings.TrimSpace(line)
			if state != "" {
				status.Nodes[state]++
				status.TotalNodes++
			}
		}
	}

	// Get job information
	squeueResult, err := s.Squeue("-h", "-o", "%T")
	if err == nil {
		lines := strings.Split(strings.TrimSpace(squeueResult.Output), "\n")
		for _, line := range lines {
			state := strings.TrimSpace(line)
			if state != "" {
				status.Jobs[state]++
				status.TotalJobs++
			}
		}
	}

	return status, nil
}

// CheckSlurmAvailability checks if SLURM commands are available
func (s *SlurmClient) CheckSlurmAvailability() error {
	commands := []string{"sinfo", "scontrol", "squeue", "sstat", "sacct", "sacctmgr", "sreport"}

	var missingCommands []string
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			missingCommands = append(missingCommands, cmd)
		}
	}

	if len(missingCommands) > 0 {
		return fmt.Errorf("SLURM commands not found: %s", strings.Join(missingCommands, ", "))
	}

	s.logger.Success("All SLURM commands are available")
	return nil
}

// Sbatch submits a batch script to SLURM
func (s *SlurmClient) Sbatch(scriptPath string, args ...string) (*CommandResult, error) {
	s.logger.Debug("Submitting batch job: %s", scriptPath)
	allArgs := append([]string{scriptPath}, args...)
	return s.executeCommand("sbatch", allArgs...)
}

// Scancel cancels a SLURM job
func (s *SlurmClient) Scancel(jobID string) (*CommandResult, error) {
	s.logger.Debug("Cancelling job: %s", jobID)
	return s.executeCommand("scancel", jobID)
}

// ToJSON converts CommandResult to JSON string
func (r *CommandResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(data), nil
}

// FormatOutput returns formatted output for display
func (r *CommandResult) FormatOutput() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Command: %s\n", r.Command))
	sb.WriteString(fmt.Sprintf("Output:\n%s", r.Output))
	if r.Error != nil {
		sb.WriteString(fmt.Sprintf("\nError: %v", r.Error))
	}
	return sb.String()
}
