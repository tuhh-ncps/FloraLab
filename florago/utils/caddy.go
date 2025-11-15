package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CaddyInstaller handles Caddy proxy installation
type CaddyInstaller struct {
	logger *Logger
}

// NewCaddyInstaller creates a new Caddy installer
func NewCaddyInstaller(logger *Logger) *CaddyInstaller {
	return &CaddyInstaller{
		logger: logger,
	}
}

// InstallCaddy builds and installs Caddy from source using xcaddy
func (c *CaddyInstaller) InstallCaddy() error {
	floragoBinDir, err := GetFloraGoBinDir()
	if err != nil {
		return fmt.Errorf("failed to get bin directory: %w", err)
	}

	// Ensure bin directory exists
	if err := CreateDirectory(floragoBinDir); err != nil {
		return fmt.Errorf("failed to create bin directory: %w", err)
	}

	caddyPath := filepath.Join(floragoBinDir, "caddy")

	// Check if Caddy already exists
	if _, err := os.Stat(caddyPath); err == nil {
		c.logger.Info("Caddy already installed at: %s", caddyPath)
		// Verify it works
		cmd := exec.Command(caddyPath, "version")
		if output, err := cmd.Output(); err == nil {
			c.logger.Success("Caddy version: %s", string(output))
			return nil
		}
	}

	c.logger.Info("Installing Caddy from source...")

	// Check if Go is available
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go is not installed or not in PATH - required to build Caddy")
	}

	// Install xcaddy if not already installed
	c.logger.Info("Installing xcaddy build tool...")
	xcaddyCmd := exec.Command("go", "install", "github.com/caddyserver/xcaddy/cmd/xcaddy@latest")
	xcaddyCmd.Env = append(os.Environ(), fmt.Sprintf("GOBIN=%s", floragoBinDir))
	if output, err := xcaddyCmd.CombinedOutput(); err != nil {
		c.logger.Debug("xcaddy install output: %s", string(output))
		return fmt.Errorf("failed to install xcaddy: %w", err)
	}

	xcaddyPath := filepath.Join(floragoBinDir, "xcaddy")

	// Build Caddy using xcaddy
	c.logger.Info("Building Caddy (this may take a few minutes)...")

	floragoTmpDir, err := GetFloraGoTempDir()
	if err != nil {
		return fmt.Errorf("failed to get temp directory: %w", err)
	}

	buildDir := filepath.Join(floragoTmpDir, "caddy-build")
	if err := CreateDirectory(buildDir); err != nil {
		return fmt.Errorf("failed to create build directory: %w", err)
	}

	// Use xcaddy to build Caddy
	buildCmd := exec.Command(xcaddyPath, "build", "--output", caddyPath)
	buildCmd.Dir = buildDir
	buildCmd.Env = append(os.Environ(),
		fmt.Sprintf("GOOS=%s", runtime.GOOS),
		fmt.Sprintf("GOARCH=%s", runtime.GOARCH),
	)

	c.logger.Info("Building Caddy for %s/%s...", runtime.GOOS, runtime.GOARCH)

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		c.logger.Debug("Build output: %s", string(output))
		return fmt.Errorf("failed to build Caddy: %w\nOutput: %s", err, string(output))
	}

	// Verify the binary was created
	if _, err := os.Stat(caddyPath); err != nil {
		return fmt.Errorf("Caddy binary not found after build: %w", err)
	}

	// Make executable
	if err := os.Chmod(caddyPath, 0755); err != nil {
		return fmt.Errorf("failed to make Caddy executable: %w", err)
	}

	// Test the binary
	versionCmd := exec.Command(caddyPath, "version")
	if versionOutput, err := versionCmd.Output(); err == nil {
		c.logger.Success("Caddy installed successfully: %s", string(versionOutput))
	} else {
		c.logger.Warning("Caddy installed but version check failed: %v", err)
	}

	c.logger.Info("Caddy binary: %s", caddyPath)

	return nil
}

// GetCaddyPath returns the path to the Caddy binary
func (c *CaddyInstaller) GetCaddyPath() (string, error) {
	floragoBinDir, err := GetFloraGoBinDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(floragoBinDir, "caddy"), nil
}

// VerifyCaddy checks if Caddy is installed and working
func (c *CaddyInstaller) VerifyCaddy() bool {
	caddyPath, err := c.GetCaddyPath()
	if err != nil {
		return false
	}

	if _, err := os.Stat(caddyPath); err != nil {
		return false
	}

	cmd := exec.Command(caddyPath, "version")
	return cmd.Run() == nil
}

// ReloadCaddy reloads the Caddy configuration
func (c *CaddyInstaller) ReloadCaddy() error {
	caddyPath, err := c.GetCaddyPath()
	if err != nil {
		return fmt.Errorf("failed to get Caddy path: %w", err)
	}

	c.logger.Info("Reloading Caddy configuration...")

	cmd := exec.Command(caddyPath, "reload")

	// Set the config file location
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return fmt.Errorf("failed to get florago home: %w", err)
	}

	caddyfileDir := filepath.Join(floragoHome, "config")
	cmd.Dir = caddyfileDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Debug("Reload output: %s", string(output))
		return fmt.Errorf("failed to reload Caddy: %w\nOutput: %s", err, string(output))
	}

	c.logger.Success("Caddy configuration reloaded")
	return nil
}

// StartCaddy starts Caddy with the Caddyfile
func (c *CaddyInstaller) StartCaddy() error {
	caddyPath, err := c.GetCaddyPath()
	if err != nil {
		return fmt.Errorf("failed to get Caddy path: %w", err)
	}

	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return fmt.Errorf("failed to get florago home: %w", err)
	}

	caddyfileDir := filepath.Join(floragoHome, "config")
	caddyfilePath := filepath.Join(caddyfileDir, "Caddyfile")

	c.logger.Info("Starting Caddy with config: %s", caddyfilePath)

	cmd := exec.Command(caddyPath, "run", "--config", caddyfilePath)
	cmd.Dir = caddyfileDir

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Caddy: %w", err)
	}

	c.logger.Success("Caddy started")
	return nil
}

// StopCaddy stops the running Caddy process
func (c *CaddyInstaller) StopCaddy() error {
	caddyPath, err := c.GetCaddyPath()
	if err != nil {
		return fmt.Errorf("failed to get Caddy path: %w", err)
	}

	c.logger.Info("Stopping Caddy...")

	cmd := exec.Command(caddyPath, "stop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		c.logger.Debug("Stop output: %s", string(output))
		return fmt.Errorf("failed to stop Caddy: %w", err)
	}

	c.logger.Success("Caddy stopped")
	return nil
}

// GetCaddyfileTemplate returns a basic Caddyfile template
func GetCaddyfileTemplate() string {
	return `{
	# Global options
	admin localhost:2019
	auto_https off
}
`
}

// CreateDefaultCaddyfile creates a default Caddyfile in the config directory
func (c *CaddyInstaller) CreateDefaultCaddyfile() error {
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return fmt.Errorf("failed to get florago home: %w", err)
	}

	configDir := filepath.Join(floragoHome, "config")
	caddyfilePath := filepath.Join(configDir, "Caddyfile")

	// Check if Caddyfile already exists
	if _, err := os.Stat(caddyfilePath); err == nil {
		c.logger.Info("Caddyfile already exists at: %s", caddyfilePath)
		return nil
	}

	c.logger.Info("Creating default Caddyfile...")

	template := GetCaddyfileTemplate()
	if err := WriteFile(caddyfilePath, []byte(template)); err != nil {
		return fmt.Errorf("failed to write Caddyfile: %w", err)
	}

	c.logger.Success("Created Caddyfile: %s", caddyfilePath)
	return nil
}
