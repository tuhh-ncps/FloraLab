package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"florago/utils"

	"github.com/spf13/cobra"
)

var clientAPIServerURL string

var flowerclientCmd = &cobra.Command{
	Use:   "flowerclient",
	Short: "Start Flower client stack (supernode + superexec)",
	Long: `Start the Flower client stack on a worker node.
This runs supernode and superexec (clientapp plugin) and connects to the server.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := utils.NewLogger(false)

		logger.Info("Starting Flower client stack...")

		// Get node information
		hostname, _ := os.Hostname()
		nodeID := fmt.Sprintf("client-%s", hostname)
		ip := getLocalIP()

		// Check flag first, then environment variable
		apiServerURL := clientAPIServerURL
		if apiServerURL == "" {
			apiServerURL = os.Getenv("FLORAGO_API_SERVER")
		}
		if apiServerURL == "" {
			logger.Fatal("API server URL not set (use --api-server flag or FLORAGO_API_SERVER environment variable)")
		}

		// Wait for server node to be ready
		logger.Info("Waiting for server node to be ready...")
		serverNode, err := waitForServerNode(apiServerURL, 300*time.Second)
		if err != nil {
			logger.Fatal("Server node not ready: %v", err)
		}

		logger.Success("Server node ready at %s", serverNode.IP)
		logger.Info("Connecting to Fleet API: %s:%d", serverNode.IP, serverNode.FleetAPIPort)

		// Get log directory
		homeDir, _ := os.UserHomeDir()
		logsDir, _ := utils.GetFloraGoLogsDir()
		jobID := os.Getenv("SLURM_JOB_ID")
		if jobID == "" {
			jobID = "local"
		}
		jobLogDir := fmt.Sprintf("%s/%s", logsDir, jobID)
		os.MkdirAll(jobLogDir, 0755)

		// Start supernode
		clientAppIOAPIPort := getEnvInt("FLOWER_CLIENT_APP_IO_API_PORT", 9094)

		logger.Info("Starting flower-supernode...")
		supernodeBin := fmt.Sprintf("%s/.florago/venv/flowerai-env/bin/flower-supernode", homeDir)
		supernodeCmd := exec.Command(
			supernodeBin,
			"--insecure",
			fmt.Sprintf("--superlink=%s:%d", serverNode.IP, serverNode.FleetAPIPort),
		)

		// Redirect supernode output to log file
		supernodeLogPath := fmt.Sprintf("%s/flower-supernode-%s.log", jobLogDir, hostname)
		supernodeLogFile, err := os.Create(supernodeLogPath)
		if err != nil {
			logger.Warning("Failed to create supernode log file: %v", err)
		} else {
			supernodeCmd.Stdout = supernodeLogFile
			supernodeCmd.Stderr = supernodeLogFile
			logger.Info("Supernode logs: %s", supernodeLogPath)
		}

		if err := supernodeCmd.Start(); err != nil {
			logger.Fatal("Failed to start supernode: %v", err)
		}
		logger.Success("Supernode started (PID: %d)", supernodeCmd.Process.Pid)

		// Wait for supernode to be ready
		time.Sleep(5 * time.Second)

		// Start superexec (clientapp)
		logger.Info("Starting flower-superexec (clientapp)...")
		superexecBin := fmt.Sprintf("%s/.florago/venv/flowerai-env/bin/flower-superexec", homeDir)
		superexecCmd := exec.Command(
			superexecBin,
			"--insecure",
			"--plugin-type=clientapp",
			fmt.Sprintf("--grpc-address=%s:%d", ip, clientAppIOAPIPort),
		)

		// Redirect superexec output to log file
		superexecLogPath := fmt.Sprintf("%s/flower-superexec-client-%s.log", jobLogDir, hostname)
		superexecLogFile, err := os.Create(superexecLogPath)
		if err != nil {
			logger.Warning("Failed to create superexec log file: %v", err)
		} else {
			superexecCmd.Stdout = superexecLogFile
			superexecCmd.Stderr = superexecLogFile
			logger.Info("Superexec logs: %s", superexecLogPath)
		}

		if err := superexecCmd.Start(); err != nil {
			logger.Fatal("Failed to start superexec: %v", err)
		}
		logger.Success("Superexec started (PID: %d)", superexecCmd.Process.Pid)

		// Register with API server
		clientNode := &utils.FlowerClientNode{
			NodeID:             nodeID,
			Hostname:           hostname,
			IP:                 ip,
			SupernodeAddress:   fmt.Sprintf("%s:%d", ip, 9092), // Supernode default port
			ClientAppIOAPIPort: clientAppIOAPIPort,
			SuperexecAddress:   fmt.Sprintf("%s:%d", ip, clientAppIOAPIPort),
			Status:             "starting",
			StartedAt:          time.Now(),
		}

		if err := registerClientNode(apiServerURL, clientNode); err != nil {
			logger.Fatal("Failed to register client node: %v", err)
		}

		logger.Success("Client node registered with API server")

		// Update status to ready
		time.Sleep(2 * time.Second)
		clientNode.Status = "ready"
		if err := registerClientNode(apiServerURL, clientNode); err != nil {
			logger.Warning("Failed to update client node status: %v", err)
		}

		logger.Success("Flower client stack is ready!")
		logger.Info("Supernode connected to: %s:%d", serverNode.IP, serverNode.FleetAPIPort)
		logger.Info("Superexec API: %s:%d", ip, clientAppIOAPIPort)

		// Wait for both processes to exit (they should run indefinitely)
		done := make(chan error, 2)

		go func() {
			if err := supernodeCmd.Wait(); err != nil {
				logger.Error("Supernode exited with error: %v", err)
				done <- err
			} else {
				logger.Warning("Supernode exited normally")
				done <- nil
			}
		}()

		go func() {
			if err := superexecCmd.Wait(); err != nil {
				logger.Error("Superexec exited with error: %v", err)
				done <- err
			} else {
				logger.Warning("Superexec exited normally")
				done <- nil
			}
		}()

		// Wait for either process to exit
		exitErr := <-done
		if exitErr != nil {
			logger.Fatal("Flower client stack failed: %v", exitErr)
		}
		logger.Warning("Flower client stack stopped")
	},
}

func init() {
	rootCmd.AddCommand(flowerclientCmd)
	flowerclientCmd.Flags().StringVar(&clientAPIServerURL, "api-server", "", "API server URL (overrides FLORAGO_API_SERVER environment variable)")
}

func waitForServerNode(apiServerURL string, timeout time.Duration) (*utils.FlowerServerNode, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(fmt.Sprintf("%s/api/flower/server", apiServerURL))
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			var serverNode utils.FlowerServerNode
			if err := json.Unmarshal(body, &serverNode); err == nil && serverNode.Status == "ready" {
				return &serverNode, nil
			}
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("timeout waiting for server node")
}

func registerClientNode(apiServerURL string, node *utils.FlowerClientNode) error {
	// This will be implemented to call the API endpoint
	// For now, just log
	fmt.Printf("Would register client node to %s\n", apiServerURL)
	return nil
}
