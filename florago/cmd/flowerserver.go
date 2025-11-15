package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"florago/utils"

	"github.com/spf13/cobra"
)

var (
	apiServerURL string
)

var flowerserverCmd = &cobra.Command{
	Use:   "flowerserver",
	Short: "Start Flower server stack (superlink + superexec)",
	Long: `Start the Flower server stack on the master node.
This runs superlink and superexec (serverapp plugin) and registers with the API server.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := utils.NewLogger(false)

		logger.Info("Starting Flower server stack...")

		// Get node information
		hostname, _ := os.Hostname()
		nodeID := fmt.Sprintf("server-%s", hostname)
		ip := getLocalIP()

		// Get ports from environment or use defaults
		serverAppIOAPIPort := getEnvInt("FLOWER_SERVER_APP_IO_API_PORT", 9091)
		fleetAPIPort := getEnvInt("FLOWER_FLEET_API_PORT", 9092)
		controlAPIPort := getEnvInt("FLOWER_CONTROL_API_PORT", 9093)

		// Get API server URL from flag or environment
		if apiServerURL == "" {
			apiServerURL = os.Getenv("FLORAGO_API_SERVER")
		}
		if apiServerURL == "" {
			logger.Fatal("API server URL not specified. Use --api-server flag or FLORAGO_API_SERVER environment variable")
		}

		logger.Info("API Server: %s", apiServerURL)

		// Get log directory
		homeDir, _ := os.UserHomeDir()
		logsDir, _ := utils.GetFloraGoLogsDir()
		jobID := os.Getenv("SLURM_JOB_ID")
		if jobID == "" {
			jobID = "local"
		}
		jobLogDir := fmt.Sprintf("%s/%s", logsDir, jobID)
		os.MkdirAll(jobLogDir, 0755)

		// Start superlink
		logger.Info("Starting flower-superlink...")
		superlinkBin := fmt.Sprintf("%s/.florago/venv/flowerai-env/bin/flower-superlink", homeDir)
		superlinkCmd := exec.Command(
			superlinkBin,
			"--insecure",
			"--isolation",
			"process",
		)

		// Redirect superlink output to log file
		superlinkLogPath := fmt.Sprintf("%s/flower-superlink.log", jobLogDir)
		superlinkLogFile, err := os.Create(superlinkLogPath)
		if err != nil {
			logger.Warning("Failed to create superlink log file: %v", err)
		} else {
			superlinkCmd.Stdout = superlinkLogFile
			superlinkCmd.Stderr = superlinkLogFile
			logger.Info("Superlink logs: %s", superlinkLogPath)
		}

		if err := superlinkCmd.Start(); err != nil {
			logger.Fatal("Failed to start superlink: %v", err)
		}
		logger.Success("Superlink started (PID: %d)", superlinkCmd.Process.Pid)

		// Wait for superlink to be ready
		time.Sleep(5 * time.Second)

		// Start superexec (serverapp)
		logger.Info("Starting flower-superexec (serverapp)...")
		superexecBin := fmt.Sprintf("%s/.florago/venv/flowerai-env/bin/flower-superexec", homeDir)
		superexecCmd := exec.Command(
			superexecBin,
			"--insecure",
			"--plugin-type=serverapp",
			fmt.Sprintf("--appio-api-address=%s:%d", ip, serverAppIOAPIPort),
		)

		// Redirect superexec output to log file
		superexecLogPath := fmt.Sprintf("%s/flower-superexec-server.log", jobLogDir)
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
		serverNode := &utils.FlowerServerNode{
			NodeID:             nodeID,
			Hostname:           hostname,
			IP:                 ip,
			SuperlinkAddress:   fmt.Sprintf("%s:%d", ip, fleetAPIPort),
			ServerAppIOAPIPort: serverAppIOAPIPort,
			FleetAPIPort:       fleetAPIPort,
			ControlAPIPort:     controlAPIPort,
			SuperexecAddress:   fmt.Sprintf("%s:%d", ip, serverAppIOAPIPort),
			Status:             "starting",
			StartedAt:          time.Now(),
		}

		if err := registerServerNode(apiServerURL, serverNode); err != nil {
			logger.Fatal("Failed to register server node: %v", err)
		}

		logger.Success("Server node registered with API server")

		// Update status to ready
		time.Sleep(2 * time.Second)
		serverNode.Status = "ready"
		if err := registerServerNode(apiServerURL, serverNode); err != nil {
			logger.Warning("Failed to update server node status: %v", err)
		}

		logger.Success("Flower server stack is ready!")
		logger.Info("Superlink Fleet API: %s:%d", ip, fleetAPIPort)
		logger.Info("Superexec API: %s:%d", ip, serverAppIOAPIPort)

		// Wait for both processes to exit (they should run indefinitely)
		done := make(chan error, 2)

		go func() {
			if err := superlinkCmd.Wait(); err != nil {
				logger.Error("Superlink exited with error: %v", err)
				done <- err
			} else {
				logger.Warning("Superlink exited normally")
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
			logger.Fatal("Flower server stack failed: %v", exitErr)
		}
		logger.Warning("Flower server stack stopped")
	},
}

func init() {
	rootCmd.AddCommand(flowerserverCmd)
	flowerserverCmd.Flags().StringVar(&apiServerURL, "api-server", "", "API server URL (can also use FLORAGO_API_SERVER env var)")
}

func getLocalIP() string {
	// In cluster environment, get hostname and resolve to LAN IP
	hostname, err := os.Hostname()
	if err != nil {
		return "127.0.0.1"
	}

	// Try to resolve hostname to IP
	addrs, err := net.LookupHost(hostname)
	if err != nil || len(addrs) == 0 {
		// If resolution fails, return hostname itself (SLURM nodes can resolve it)
		return hostname
	}

	// Return first resolved IP address
	return addrs[0]
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func registerServerNode(apiServerURL string, node *utils.FlowerServerNode) error {
	// Prepare registration payload
	payload := map[string]interface{}{
		"ip":                     node.IP,
		"server_app_io_api_port": node.ServerAppIOAPIPort,
		"fleet_api_port":         node.FleetAPIPort,
		"control_api_port":       node.ControlAPIPort,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration data: %w", err)
	}

	// Send POST request to register server node
	url := fmt.Sprintf("%s/api/flower/server", apiServerURL)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send registration request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}
