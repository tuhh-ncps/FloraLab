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

		apiServerURL := os.Getenv("FLORAGO_API_SERVER")
		if apiServerURL == "" {
			logger.Fatal("FLORAGO_API_SERVER environment variable not set")
		}

		// Wait for server node to be ready
		logger.Info("Waiting for server node to be ready...")
		serverNode, err := waitForServerNode(apiServerURL, 300*time.Second)
		if err != nil {
			logger.Fatal("Server node not ready: %v", err)
		}

		logger.Success("Server node ready at %s", serverNode.IP)
		logger.Info("Connecting to Fleet API: %s:%d", serverNode.IP, serverNode.FleetAPIPort)

		// Start supernode
		clientAppIOAPIPort := getEnvInt("FLOWER_CLIENT_APP_IO_API_PORT", 9094)

		logger.Info("Starting flower-supernode...")
		homeDir, _ := os.UserHomeDir()
		supernodeBin := fmt.Sprintf("%s/.florago/venv/flowerai-env/bin/flower-supernode", homeDir)
		supernodeCmd := exec.Command(
			supernodeBin,
			"--insecure",
			fmt.Sprintf("--superlink=%s:%d", serverNode.IP, serverNode.FleetAPIPort),
		)

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

		// Keep running
		select {}
	},
}

func init() {
	rootCmd.AddCommand(flowerclientCmd)
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
