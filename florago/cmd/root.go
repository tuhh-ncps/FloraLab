package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"florago/utils"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "florago",
	Short: "FloraGo - Federated Learning orchestration on SLURM clusters",
	Long: `FloraGo is a CLI tool for managing Flower-AI federated learning stacks on SLURM clusters.
It provides simple commands to initialize environments, start API servers, and orchestrate
distributed Flower deployments across compute nodes.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("FloraGo %s\n", version)
		fmt.Printf("Commit: %s\n", commit)
		fmt.Printf("Built: %s\n", date)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize FloraGo environment",
	Long: `Initialize FloraGo environment in $HOME/.florago.
This will set up the directory structure, Python virtual environment, and install required packages.

All FloraGo data is stored in $HOME/.florago to ensure compatibility with SLURM login nodes.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := utils.NewLogger(false)

		// Get FloraGo home directory
		floragoHome, err := utils.GetFloraGoHome()
		if err != nil {
			logger.Fatal("Failed to get FloraGo home directory: %v", err)
		}

		logger.Info("Initializing FloraGo in: %s", floragoHome)

		// Check system requirements
		logger.Info("\n🔍 Checking system requirements...")

		// Check Python
		pythonInfo, err := utils.CheckPython()
		if err != nil {
			logger.Fatal("Error checking Python: %v", err)
		}

		if !pythonInfo.Available {
			logger.Fatal("Python 3 is not installed or not found in PATH")
		}

		logger.Success("Python: %s", pythonInfo.GetVersionString())
		logger.Info("  Source: %s", pythonInfo.GetSourceString())
		logger.Info("  Path: %s", pythonInfo.Path)

		if !pythonInfo.IsPythonVersionSupported() {
			logger.Fatal("Python version must be >= 3.11 (found %s)", pythonInfo.GetVersionString())
		}

		logger.Success("  ✓ Python version is >= 3.11")

		// Check Environment Modules
		moduleInfo, err := utils.CheckModules()
		if err != nil {
			logger.Warning("Error checking modules: %v", err)
		} else {
			if moduleInfo.Available {
				logger.Success("Environment Modules: %s", moduleInfo.GetTypeString())
				if moduleInfo.Version != "" {
					logger.Info("  Version: %s", moduleInfo.Version)
				}
				logger.Info("  ✓ Module system available for software management")
			} else {
				logger.Warning("Environment Modules: Not found")
				logger.Info("  Environment Modules (Lmod/TCL) not detected")
				logger.Info("  This is optional but useful for HPC environments")
			}
		}

		logger.Info("\n📁 Setting up FloraGo directory structure...")

		// Create FloraGo directories in $HOME/.florago
		floragoDataDir, err := utils.GetFloraGoDataDir()
		if err != nil {
			logger.Fatal("Failed to get data directory: %v", err)
		}

		logsDir := filepath.Join(floragoHome, "logs")
		configDir := filepath.Join(floragoHome, "config")

		dirs := []string{floragoDataDir, logsDir, configDir}
		for _, dir := range dirs {
			if err := utils.CreateDirectory(dir); err != nil {
				logger.Fatal("Failed to create directory %s: %v", dir, err)
			}
			logger.Success("Created directory: %s", dir)
		}

		// Create Python virtual environment
		logger.Info("\n🐍 Setting up Python virtual environment...")

		venvPath, err := utils.GetFlowerAIVenvPath()
		if err != nil {
			logger.Fatal("Failed to get venv path: %v", err)
		}

		venvManager := utils.NewVenvManager(pythonInfo.Path, logger)

		// Check if venv already exists
		venvCreated := false
		if venvManager.VerifyVenv(venvPath) {
			logger.Success("Virtual environment already exists: %s", venvPath)
			venvManager.SetVenvPath(venvPath)
		} else {
			if err := venvManager.CreateVenv(venvPath); err != nil {
				logger.Fatal("Failed to create virtual environment: %v", err)
			}
			logger.Success("Virtual environment created: %s", venvPath)
			venvCreated = true
		}

		logger.Info("  Python: %s", venvManager.GetVenvPythonPath())
		logger.Info("  Activate: source %s", venvManager.GetVenvActivateScript())

		// Install Flower (flwr) package
		logger.Info("\n📦 Installing Python packages...")

		if venvCreated {
			// Upgrade pip first for new venvs
			if err := venvManager.UpgradePip(); err != nil {
				logger.Warning("Failed to upgrade pip: %v", err)
			}
		}

		// Install OpenSSL 3 locally if not already present
		// This is needed because cryptography package requires OpenSSL 3.x
		// but many HPC systems only have OpenSSL 1.1.1
		logger.Info("Checking for OpenSSL 3...")
		opensslDir := filepath.Join(floragoHome, "openssl3")

		// Check if OpenSSL 3 is already installed
		opensslLib := filepath.Join(opensslDir, "lib", "libssl.so.3")
		if _, err := os.Stat(opensslLib); err == nil {
			logger.Success("OpenSSL 3 already installed: %s", opensslDir)
		} else {
			logger.Info("OpenSSL 3 not found, installing locally (this will take 5-10 minutes)...")

			// Download and build OpenSSL 3
			tmpDir := filepath.Join(floragoHome, "tmp")
			os.MkdirAll(tmpDir, 0755)

			opensslTarball := filepath.Join(tmpDir, "openssl-3.2.1.tar.gz")
			opensslSrcDir := filepath.Join(tmpDir, "openssl-3.2.1")

			// Download OpenSSL 3.2.1
			logger.Info("Downloading OpenSSL 3.2.1...")
			downloadCmd := exec.Command("wget", "-O", opensslTarball, "https://www.openssl.org/source/openssl-3.2.1.tar.gz")
			downloadCmd.Dir = tmpDir
			if output, err := downloadCmd.CombinedOutput(); err != nil {
				logger.Fatal("Failed to download OpenSSL: %v\n%s", err, output)
			}

			// Extract tarball
			logger.Info("Extracting OpenSSL...")
			extractCmd := exec.Command("tar", "xzf", opensslTarball)
			extractCmd.Dir = tmpDir
			if output, err := extractCmd.CombinedOutput(); err != nil {
				logger.Fatal("Failed to extract OpenSSL: %v\n%s", err, output)
			}

			// Configure OpenSSL
			logger.Info("Configuring OpenSSL (this may take a few minutes)...")
			configCmd := exec.Command("./config",
				fmt.Sprintf("--prefix=%s", opensslDir),
				fmt.Sprintf("--openssldir=%s", opensslDir))
			configCmd.Dir = opensslSrcDir
			if output, err := configCmd.CombinedOutput(); err != nil {
				logger.Fatal("Failed to configure OpenSSL: %v\n%s", err, output)
			}

			// Build OpenSSL
			logger.Info("Building OpenSSL (this will take 5-10 minutes)...")
			makeCmd := exec.Command("make", "-j8")
			makeCmd.Dir = opensslSrcDir
			if output, err := makeCmd.CombinedOutput(); err != nil {
				logger.Fatal("Failed to build OpenSSL: %v\n%s", err, output)
			}

			// Install OpenSSL
			logger.Info("Installing OpenSSL to %s...", opensslDir)
			installCmd := exec.Command("make", "install")
			installCmd.Dir = opensslSrcDir
			if output, err := installCmd.CombinedOutput(); err != nil {
				logger.Fatal("Failed to install OpenSSL: %v\n%s", err, output)
			}

			// Clean up
			logger.Info("Cleaning up temporary files...")
			os.RemoveAll(tmpDir)

			logger.Success("OpenSSL 3 installed successfully")
		}

		// First install cryptography with OpenSSL 3
		// Set environment variables to use the local OpenSSL 3
		logger.Info("Installing cryptography with OpenSSL 3...")
		cryptoPackages := []string{"cryptography"}
		cryptoFlags := []string{"--no-binary", "cryptography", "--no-cache-dir"}
		cryptoEnvVars := []string{
			fmt.Sprintf("LD_LIBRARY_PATH=%s/lib:$LD_LIBRARY_PATH", opensslDir),
			fmt.Sprintf("LIBRARY_PATH=%s/lib:$LIBRARY_PATH", opensslDir),
			fmt.Sprintf("CPATH=%s/include:$CPATH", opensslDir),
		}
		if err := venvManager.InstallPackagesWithFlags(cryptoPackages, cryptoFlags, cryptoEnvVars); err != nil {
			logger.Fatal("Failed to install cryptography: %v", err)
		}

		// Then install flwr and ray (which will use the already-installed cryptography)
		logger.Info("Installing flwr[simulation] and ray...")
		packages := []string{"flwr[simulation]", "ray"}
		if err := venvManager.InstallPackages(packages); err != nil {
			logger.Fatal("Failed to install packages: %v", err)
		}

		// Skip Caddy installation - it will be copied by floralab-cli
		logger.Info("\n🌐 Preparing Caddy configuration...")
		caddyInstaller := utils.NewCaddyInstaller(logger)

		// Create default Caddyfile
		if err := caddyInstaller.CreateDefaultCaddyfile(); err != nil {
			logger.Fatal("Failed to create Caddyfile: %v", err)
		}
		logger.Info("  Caddyfile created (Caddy binary will be provided by floralab-cli)")

		// Skip Delve installation - it will be copied by floralab-cli
		logger.Info("\n🐛 Delve debugger will be provided by floralab-cli")

		// Create config file with venv info
		config := utils.DefaultConfig("florago")
		config.SetVenv(
			"flowerai",
			venvPath,
			venvManager.GetVenvPythonPath(),
			venvManager.GetVenvActivateScript(),
		)

		configJSON, err := config.ToJSON()
		if err != nil {
			logger.Fatal("Failed to generate config: %v", err)
		}

		configPath := filepath.Join(configDir, "florago.json")
		if err := utils.WriteFile(configPath, []byte(configJSON)); err != nil {
			logger.Fatal("Failed to write config file: %v", err)
		}
		logger.Success("Created configuration file: %s", configPath)

		logger.Info("\n✨ FloraGo initialized successfully!")
		logger.Info("FloraGo home: %s", floragoHome)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
