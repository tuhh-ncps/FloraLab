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
	var req SpinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// Validate request
	if req.NumNodes < 1 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "num_nodes must be at least 1",
		})
		return
	}

	// Check if a stack is already running
	if stackManager.IsStackRunning() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: "A Flower stack is already running",
			State:   stackManager.GetState(),
		})
		return
	}

	// Parse job ID first (we'll get it after sbatch, but initialize with empty for now)
	// We'll update with real jobID after submission

	// Create SLURM job script for distributed Flower stack
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

	// Write script to temp file
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

	// Submit job
	result, err := slurmClient.Sbatch(scriptPath)
	if err != nil {
		logger.Error("Failed to submit job: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(SpinResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to submit job: %v", err),
		})
		stackManager.ClearState()
		return
	}

	// Parse job ID
	jobID := parseJobID(result.Output)
	currentJobID = jobID

	// Initialize stack with the job ID
	stackManager.InitializeStack(jobID, req.NumNodes)

	logger.Success("Flower stack job submitted: %s", jobID)
	logger.Info("Waiting for %d client nodes + 1 server node to register...", req.NumNodes)

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
			// Server registration
			var req ServerRegisterRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
				return
			}

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

			err := stackManager.RegisterServerNode(serverNode)
			if err != nil {
				logger.Error("Failed to register server: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
				return
			}

			logger.Success("Server node registered: %s", req.IP)
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

		var req ClientRegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
			return
		}

		// Create FlowerClientNode struct
		clientNode := &utils.FlowerClientNode{
			NodeID:    fmt.Sprintf("client-%s", req.IP),
			IP:        req.IP,
			Status:    "ready",
			StartedAt: time.Now(),
		}

		err := stackManager.RegisterClientNode(clientNode)
		if err != nil {
			logger.Error("Failed to register client: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		logger.Success("Client node registered: %s", req.IP)
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
	floragoPath, err := utils.GetFloraGoBinDir()
	if err != nil {
		return "", err
	}

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

	// Get florago binary path
	script += fmt.Sprintf("FLORAGO_BIN=%s/florago\n\n", floragoPath)

	// Launch commands in parallel using srun
	script += "# Launch server on first node\n"
	script += "srun --nodes=1 --ntasks=1 --nodelist=$(scontrol show hostname $SLURM_JOB_NODELIST | head -n 1) \\\n"
	script += "  $FLORAGO_BIN flowerserver --api-server $FLORAGO_API_SERVER &\n\n"

	script += "# Launch clients on remaining nodes\n"
	script += "if [ $SLURM_NNODES -gt 1 ]; then\n"
	script += "  CLIENT_NODES=$(scontrol show hostname $SLURM_JOB_NODELIST | tail -n +2)\n"
	script += "  for node in $CLIENT_NODES; do\n"
	script += "    srun --nodes=1 --ntasks=1 --nodelist=$node \\\n"
	script += "      $FLORAGO_BIN flowerclient --api-server $FLORAGO_API_SERVER &\n"
	script += "  done\n"
	script += "fi\n\n"

	script += "# Wait for all background processes\n"
	script += "wait\n"

	return script, nil
}
