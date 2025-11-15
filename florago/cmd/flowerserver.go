package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"time"

	"florago/utils"

	"github.com/spf13/cobra"
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
		apiServerURL := os.Getenv("FLORAGO_API_SERVER")
		if apiServerURL == "" {
			logger.Fatal("FLORAGO_API_SERVER environment variable not set")
		}

		// Start superlink
		logger.Info("Starting flower-superlink...")
		homeDir, _ := os.UserHomeDir()
		superlinkBin := fmt.Sprintf("%s/.florago/venv/flowerai-env/bin/flower-superlink", homeDir)
		superlinkCmd := exec.Command(
			superlinkBin,
			"--insecure",
			fmt.Sprintf("--grpc-bidi-address=%s:%d", ip, fleetAPIPort),
		)

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
			fmt.Sprintf("--grpc-address=%s:%d", ip, serverAppIOAPIPort),
		)

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

		// Keep running
		select {}
	},
}

func init() {
	rootCmd.AddCommand(flowerserverCmd)
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
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
	// This will be implemented to call the API endpoint
	// For now, just log
	fmt.Printf("Would register server node to %s\n", apiServerURL)
	return nil
}
