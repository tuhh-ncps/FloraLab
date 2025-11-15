package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"florago/utils"

	"github.com/spf13/cobra"
)

var (
	serverPort   string
	serverHost   string
	stackManager *utils.FlowerStackManager
	currentJobID string // Track the currently running Flower stack job
)

// SpinRequest represents a request to spin up a Flower stack
type SpinRequest struct {
	NumNodes  int    `json:"num_nodes"`            // Number of client nodes
	Partition string `json:"partition,omitempty"`  // SLURM partition
	Memory    string `json:"memory,omitempty"`     // Memory per node (e.g., "4G")
	TimeLimit string `json:"time_limit,omitempty"` // Time limit (e.g., "01:00:00")
}

// SpinResponse represents the response from spin endpoint
type SpinResponse struct {
	Success bool                    `json:"success"`
	JobID   string                  `json:"job_id,omitempty"`
	Message string                  `json:"message"`
	State   *utils.FlowerStackState `json:"state,omitempty"`
}

// MonitoringResponse represents comprehensive cluster and stack status
type MonitoringResponse struct {
	Timestamp   string                  `json:"timestamp"`
	FlowerStack *utils.FlowerStackState `json:"flower_stack"`
	SlurmInfo   map[string]interface{}  `json:"slurm_info"`
}

// ServerRegisterRequest represents server node registration
type ServerRegisterRequest struct {
	IP                 string `json:"ip"`
	ServerAppIOAPIPort int    `json:"server_app_io_api_port"`
	FleetAPIPort       int    `json:"fleet_api_port"`
	ControlAPIPort     int    `json:"control_api_port"`
}

// ClientRegisterRequest represents client node registration
type ClientRegisterRequest struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start FloraGo HTTP server",
	Long: `Start the FloraGo HTTP REST API server.
This server provides endpoints for managing Flower-AI stacks on SLURM:
  - GET  /health                - Health check
  - GET  /api/monitoring        - Get comprehensive stack and cluster status
  - POST /api/spin              - Spin up Flower-AI stack
  - GET  /api/spin              - Get current stack status
  - DELETE /api/spin            - Tear down Flower-AI stack

Internal coordination endpoints (used by florago nodes):
  - POST /api/flower/server     - Server node registration
  - POST /api/flower/client     - Client node registration
  - GET  /api/flower/server     - Get server info (for clients to connect)

The server runs in the foreground and can be stopped with Ctrl+C.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := utils.NewLogger(false)

		logger.Info("Starting FloraGo HTTP server...")
		logger.Info("Host: %s", serverHost)
		logger.Info("Port: %s", serverPort)

		// Initialize stack manager
		stackManager = utils.NewFlowerStackManager(logger)

		// Initialize SLURM client
		slurmClient := utils.NewSlurmClient(logger)

		// Initialize and start Caddy
		logger.Info("Starting Caddy reverse proxy...")
		caddyInstaller := utils.NewCaddyInstaller(logger)

		// Ensure Caddy is installed
		if !caddyInstaller.VerifyCaddy() {
			logger.Warning("Caddy not found - reverse proxy will not be available")
			logger.Info("Run 'florago init' to install Caddy")
		} else {
			// Start Caddy in the background
			if err := caddyInstaller.StartCaddy(); err != nil {
				logger.Warning("Failed to start Caddy: %v", err)
				logger.Warning("Reverse proxy will not be available")
			} else {
				logger.Success("Caddy reverse proxy started")
			}
		}

		// Check if SLURM is available
		err := slurmClient.CheckSlurmAvailability()
		if err != nil {
			logger.Warning("SLURM not detected - some features may not work")
		} else {
			logger.Success("SLURM cluster detected")
		}

		// Setup HTTP routes - 3 main endpoints + coordination endpoints
		http.HandleFunc("/health", handleHealth)
		http.HandleFunc("/api/monitoring", makeMonitoringHandler(slurmClient, logger))
		http.HandleFunc("/api/spin", makeSpinHandler(slurmClient, logger))

		// Internal coordination endpoints
		http.HandleFunc("/api/flower/server", makeFlowerServerHandler(logger))
		http.HandleFunc("/api/flower/client", makeFlowerClientHandler(logger))

		addr := fmt.Sprintf("%s:%s", serverHost, serverPort)
		logger.Success("Server ready at http://%s", addr)
		logger.Info("\nAvailable endpoints:")
		logger.Info("  GET  /health                - Health check")
		logger.Info("  GET  /api/monitoring        - Get stack and cluster status")
		logger.Info("  POST /api/spin              - Spin up Flower-AI stack")
		logger.Info("  GET  /api/spin              - Get current stack status")
		logger.Info("  DELETE /api/spin            - Tear down Flower-AI stack")
		logger.Info("\nPress Ctrl+C to stop the server")

		// Start server
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Fatal("Server failed to start: %v", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVar(&serverPort, "port", "8080", "Server port")
	startCmd.Flags().StringVar(&serverHost, "host", "0.0.0.0", "Server host")
}

// handleHealth serves the health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// makeMonitoringHandler returns the monitoring endpoint handler
func makeMonitoringHandler(slurmClient *utils.SlurmClient, logger *utils.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get Flower stack state
		flowerState := stackManager.GetState()

		// Get SLURM cluster info
		slurmInfo := make(map[string]interface{})

		// Get node info
		if nodeResult, err := slurmClient.Sinfo("-N", "-o", "%N %T %C %m %e %f"); err == nil {
			slurmInfo["nodes"] = nodeResult.Output
		}

		// Get job info for current user
		username := os.Getenv("USER")
		if jobResult, err := slurmClient.Squeue("-u", username, "-o", "%.18i %.9P %.30j %.8T %.10M %.6D %R"); err == nil {
			slurmInfo["jobs"] = jobResult.Output
			slurmInfo["user"] = username
		}

		// If we have a current job ID, get detailed info
		if currentJobID != "" {
			if jobDetailResult, err := slurmClient.Scontrol("show", "job", currentJobID); err == nil {
				slurmInfo["current_job_detail"] = jobDetailResult.Output
			}
		}

		response := MonitoringResponse{
			Timestamp:   time.Now().Format(time.RFC3339),
			FlowerStack: flowerState,
			SlurmInfo:   slurmInfo,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// makeSpinHandler returns the spin endpoint handler (POST/GET/DELETE)
func makeSpinHandler(slurmClient *utils.SlurmClient, logger *utils.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handleSpinUp(w, r, slurmClient, logger)
		case http.MethodGet:
			handleSpinStatus(w, r, logger)
		case http.MethodDelete:
			handleSpinDown(w, r, slurmClient, logger)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// handleSpinUp starts a new Flower stack
func handleSpinUp(w http.ResponseWriter, r *http.Request, slurmClient *utils.SlurmClient, logger *utils.Logger) {
	logger.Info("=== POST /api/spin - Spin up Flower stack ===")
	logger.Info("Request from: %s", r.RemoteAddr)

	var req SpinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("Failed to decode request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	logger.Info("Request parameters:")
	logger.Info("  NumNodes: %d", req.NumNodes)
	logger.Info("  Partition: %s", req.Partition)
	logger.Info("  Memory: %s", req.Memory)
	logger.Info("  TimeLimit: %s", req.TimeLimit)

	// Validate request
	if req.NumNodes < 1 {
		logger.Error("Invalid num_nodes: %d (must be >= 1)", req.NumNodes)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "num_nodes must be at least 1",
		})
		return
	}

	// Check if a stack is already running
	logger.Info("Checking if stack is already running...")
	if stackManager.IsStackRunning() {
		logger.Warning("Stack already running - rejecting request")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "A Flower stack is already running",
			State:   stackManager.GetState(),
		})
		return
	}
	logger.Info("No existing stack - proceeding with spin up")

	// Parse job ID first (we'll get it after sbatch, but initialize with empty for now)
	// We'll update with real jobID after submission

	// Create SLURM job script for distributed Flower stack
	logger.Info("Creating SLURM job script...")
	jobScript, err := createFlowerStackScript(req, serverHost, serverPort)
	if err != nil {
		logger.Error("Failed to create job script: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "Failed to create job script",
		})
		stackManager.ClearState()
		return
	}
	logger.Debug("Job script created successfully (%d bytes)", len(jobScript))

	// Write script to temp file
	logger.Info("Writing job script to temp file...")
	floragoTmpDir, err := utils.GetFloraGoTempDir()
	if err != nil {
		logger.Error("Failed to get temp directory: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "Failed to access temp directory",
		})
		stackManager.ClearState()
		return
	}

	scriptPath := filepath.Join(floragoTmpDir, fmt.Sprintf("flower_stack_%d.sh", time.Now().Unix()))
	logger.Info("Script path: %s", scriptPath)
	if err := utils.WriteFile(scriptPath, []byte(jobScript)); err != nil {
		logger.Error("Failed to write job script: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "Failed to write job script",
		})
		stackManager.ClearState()
		return
	}
	logger.Success("Job script written to: %s", scriptPath)

	// Submit job
	logger.Info("Submitting job to SLURM...")
	logger.Debug("Command: sbatch %s", scriptPath)
	result, err := slurmClient.Sbatch(scriptPath)
	if err != nil {
		logger.Error("Failed to submit job: %v", err)
		logger.Error("SLURM output: %s", result.Output)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to submit job: %v", err),
		})
		stackManager.ClearState()
		return
	}
	logger.Info("SLURM sbatch output: %s", result.Output)

	// Parse job ID
	jobID := parseJobID(result.Output)
	logger.Info("Parsed job ID: %s", jobID)
	currentJobID = jobID

	// Initialize stack with the job ID
	logger.Info("Initializing stack manager with job ID: %s", jobID)
	stackManager.InitializeStack(jobID, req.NumNodes)

	logger.Success("Flower stack job submitted: %s", jobID)
	logger.Info("Expected nodes: %d clients + 1 server = %d total", req.NumNodes, req.NumNodes+1)
	logger.Info("Waiting for nodes to register...")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpinResponse{
		Success: true,
		JobID:   jobID,
		Message: fmt.Sprintf("Flower stack job %s submitted successfully", jobID),
		State:   stackManager.GetState(),
	})
}

// handleSpinStatus returns current Flower stack status
func handleSpinStatus(w http.ResponseWriter, r *http.Request, logger *utils.Logger) {
	state := stackManager.GetState()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpinResponse{
		Success: true,
		JobID:   currentJobID,
		Message: fmt.Sprintf("Stack status: %s", state.Status),
		State:   state,
	})
}

// handleSpinDown tears down the current Flower stack
func handleSpinDown(w http.ResponseWriter, r *http.Request, slurmClient *utils.SlurmClient, logger *utils.Logger) {
	if currentJobID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "No Flower stack is currently running",
		})
		return
	}

	// Cancel SLURM job
	_, err := slurmClient.Scancel(currentJobID)
	if err != nil {
		logger.Error("Failed to cancel job %s: %v", currentJobID, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to cancel job: %v", err),
		})
		return
	}

	logger.Success("Flower stack job %s cancelled", currentJobID)

	// Clear state
	stackManager.ClearState()
	oldJobID := currentJobID
	currentJobID = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpinResponse{
		Success: true,
		JobID:   oldJobID,
		Message: fmt.Sprintf("Flower stack job %s cancelled successfully", oldJobID),
	})
}

// makeFlowerServerHandler handles server node registration (POST) and info retrieval (GET)
func makeFlowerServerHandler(logger *utils.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			logger.Info("=== POST /api/flower/server - Server node registration ===")
			logger.Info("Request from: %s", r.RemoteAddr)

			// Server registration
			var req ServerRegisterRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				logger.Error("Failed to decode server registration request: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
				return
			}

			logger.Info("Server registration details:")
			logger.Info("  IP: %s", req.IP)
			logger.Info("  ServerAppIOAPIPort: %d", req.ServerAppIOAPIPort)
			logger.Info("  FleetAPIPort: %d", req.FleetAPIPort)
			logger.Info("  ControlAPIPort: %d", req.ControlAPIPort)

			// Create FlowerServerNode struct
			serverNode := &utils.FlowerServerNode{
				NodeID:             fmt.Sprintf("server-%s", req.IP),
				IP:                 req.IP,
				ServerAppIOAPIPort: req.ServerAppIOAPIPort,
				FleetAPIPort:       req.FleetAPIPort,
				ControlAPIPort:     req.ControlAPIPort,
				Status:             "ready",
				StartedAt:          time.Now(),
			}

			logger.Info("Registering server node with stack manager...")
			err := stackManager.RegisterServerNode(serverNode)
			if err != nil {
				logger.Error("Failed to register server: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			logger.Success("Server node registered: %s (node ID: %s)", req.IP, serverNode.NodeID)
			logger.Info("Current stack state: %d/%d nodes registered",
				stackManager.GetState().CompletedNodes,
				stackManager.GetState().ExpectedNodes)

			// Configure Caddy reverse proxy for Control API
			logger.Info("Configuring reverse proxy for Control API...")
			caddyInstaller := utils.NewCaddyInstaller(logger)
			if err := caddyInstaller.ConfigureFlowerControlProxy(req.ControlAPIPort, req.IP); err != nil {
				logger.Warning("Failed to configure reverse proxy: %v", err)
				logger.Warning("Control API will only be accessible directly at %s:%d", req.IP, req.ControlAPIPort)
			} else {
				logger.Success("Control API reverse proxy: 0.0.0.0:%d -> %s:%d", req.ControlAPIPort, req.IP, req.ControlAPIPort)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "registered"})

		case http.MethodGet:
			// Get server info (for clients to connect)
			timeout := 300 * time.Second // 5 minutes timeout
			serverNode, err := stackManager.GetServerInfo(timeout)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(serverNode)

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

// makeFlowerClientHandler handles client node registration
func makeFlowerClientHandler(logger *utils.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		logger.Info("=== POST /api/flower/client - Client node registration ===")
		logger.Info("Request from: %s", r.RemoteAddr)

		var req ClientRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			logger.Error("Failed to decode client registration request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
			return
		}

		logger.Info("Client registration details:")
		logger.Info("  IP: %s", req.IP)
		logger.Info("  Port: %d", req.Port)

		// Create FlowerClientNode struct
		clientNode := &utils.FlowerClientNode{
			NodeID:    fmt.Sprintf("client-%s", req.IP),
			IP:        req.IP,
			Status:    "ready",
			StartedAt: time.Now(),
		}

		logger.Info("Registering client node with stack manager...")
		err := stackManager.RegisterClientNode(clientNode)
		if err != nil {
			logger.Error("Failed to register client: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		logger.Success("Client node registered: %s (node ID: %s)", req.IP, clientNode.NodeID)
		logger.Info("Current stack state: %d/%d nodes registered",
			stackManager.GetState().CompletedNodes,
			stackManager.GetState().ExpectedNodes)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
	}
}

// parseJobID extracts job ID from sbatch output
func parseJobID(output string) string {
	// sbatch output format: "Submitted batch job 12345"
	var jobID string
	fmt.Sscanf(output, "Submitted batch job %s", &jobID)
	return strings.TrimSpace(jobID)
}

// createFlowerStackScript generates a SLURM batch script for Flower stack deployment
func createFlowerStackScript(req SpinRequest, apiHost, apiPort string) (string, error) {
	totalNodes := req.NumNodes + 1 // +1 for server node

	script := "#!/bin/bash\n"
	script += "#SBATCH --job-name=flower-stack\n"
	script += fmt.Sprintf("#SBATCH --nodes=%d\n", totalNodes)
	script += "#SBATCH --ntasks-per-node=1\n"

	if req.Partition != "" {
		script += fmt.Sprintf("#SBATCH --partition=%s\n", req.Partition)
	}
	if req.Memory != "" {
		script += fmt.Sprintf("#SBATCH --mem=%s\n", req.Memory)
	}
	if req.TimeLimit != "" {
		script += fmt.Sprintf("#SBATCH --time=%s\n", req.TimeLimit)
	}

	// Output/error logs
	logsDir, _ := utils.GetFloraGoLogsDir()
	script += fmt.Sprintf("#SBATCH --output=%s/flower-stack-%%j.out\n", logsDir)
	script += fmt.Sprintf("#SBATCH --error=%s/flower-stack-%%j.err\n", logsDir)

	script += "\n# Flower Stack Deployment\n"
	script += "# This script deploys 1 server node + N client nodes in parallel\n\n"

	// API server environment variable
	apiURL := fmt.Sprintf("http://%s:%s", apiHost, apiPort)
	script += fmt.Sprintf("export FLORAGO_API_SERVER=%s\n\n", apiURL)

	// Get florago binary path - it's in $HOME/florago-amd64 (copied by floralab-cli)
	script += "FLORAGO_BIN=$HOME/florago-amd64\n\n"

	// Create job-specific log directory
	script += "# Create job-specific log directory\n"
	script += fmt.Sprintf("JOB_LOG_DIR=%s/${SLURM_JOB_ID}\n", logsDir)
	script += "mkdir -p $JOB_LOG_DIR\n"
	script += "echo \"Job logs will be written to: $JOB_LOG_DIR\"\n\n"

	// Launch commands in parallel using srun
	script += "# Launch server on first node\n"
	script += "srun --nodes=1 --ntasks=1 --nodelist=$(scontrol show hostname $SLURM_JOB_NODELIST | head -n 1) \\\n"
	script += "  $FLORAGO_BIN flowerserver --api-server $FLORAGO_API_SERVER \\\n"
	script += "  > $JOB_LOG_DIR/flowerserver.log 2>&1 &\n\n"

	script += "# Launch clients on remaining nodes\n"
	script += "if [ $SLURM_NNODES -gt 1 ]; then\n"
	script += "  CLIENT_NODES=$(scontrol show hostname $SLURM_JOB_NODELIST | tail -n +2)\n"
	script += "  CLIENT_INDEX=0\n"
	script += "  for node in $CLIENT_NODES; do\n"
	script += "    srun --nodes=1 --ntasks=1 --nodelist=$node \\\n"
	script += "      $FLORAGO_BIN flowerclient --api-server $FLORAGO_API_SERVER \\\n"
	script += "      > $JOB_LOG_DIR/flowerclient-${CLIENT_INDEX}.log 2>&1 &\n"
	script += "    CLIENT_INDEX=$((CLIENT_INDEX + 1))\n"
	script += "  done\n"
	script += "fi\n\n"

	script += "# Wait for all background processes\n"
	script += "wait\n"

	return script, nil
}
