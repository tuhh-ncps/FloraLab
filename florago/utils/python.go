package utils

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// PythonInfo holds information about Python installation
type PythonInfo struct {
	Available bool
	Version   string
	Major     int
	Minor     int
	Patch     int
	Path      string
	Source    string // "system", "conda", "pyenv", "virtualenv", etc.
}

// CheckPython checks if Python 3 is available and gathers information
func CheckPython() (*PythonInfo, error) {
	info := &PythonInfo{
		Available: false,
	}

	// Try to execute python3
	cmd := exec.Command("python3", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return info, nil // Python not available, not an error
	}

	info.Available = true

	// Parse version from output (e.g., "Python 3.11.5")
	versionStr := strings.TrimSpace(string(output))
	versionRegex := regexp.MustCompile(`Python (\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(versionStr)

	if len(matches) == 4 {
		info.Version = fmt.Sprintf("%s.%s.%s", matches[1], matches[2], matches[3])
		info.Major, _ = strconv.Atoi(matches[1])
		info.Minor, _ = strconv.Atoi(matches[2])
		info.Patch, _ = strconv.Atoi(matches[3])
	}

	// Get Python path
	pathCmd := exec.Command("python3", "-c", "import sys; print(sys.executable)")
	pathOutput, err := pathCmd.Output()
	if err == nil {
		info.Path = strings.TrimSpace(string(pathOutput))
	}

	// Determine Python source
	info.Source = determinePythonSource(info.Path)

	return info, nil
}

// determinePythonSource identifies where Python comes from
func determinePythonSource(pythonPath string) string {
	if pythonPath == "" {
		return "unknown"
	}

	// Check for conda
	if strings.Contains(pythonPath, "conda") || strings.Contains(pythonPath, "anaconda") || strings.Contains(pythonPath, "miniconda") {
		return "conda"
	}

	// Check for pyenv
	if strings.Contains(pythonPath, "pyenv") {
		return "pyenv"
	}

	// Check for virtualenv/venv
	if strings.Contains(pythonPath, "venv") || strings.Contains(pythonPath, "virtualenv") {
		return "virtualenv"
	}

	// Check for homebrew (macOS)
	if strings.Contains(pythonPath, "homebrew") || strings.Contains(pythonPath, "Cellar") {
		return "homebrew"
	}

	// Check for system paths
	systemPaths := []string{"/usr/bin", "/usr/local/bin", "/bin"}
	for _, sysPath := range systemPaths {
		if strings.HasPrefix(pythonPath, sysPath) {
			return "system"
		}
	}

	return "custom"
}

// IsPythonVersionSupported checks if Python version is >= 3.11
func (p *PythonInfo) IsPythonVersionSupported() bool {
	if !p.Available {
		return false
	}
	if p.Major > 3 {
		return true
	}
	if p.Major == 3 && p.Minor >= 11 {
		return true
	}
	return false
}

// GetVersionString returns a formatted version string
func (p *PythonInfo) GetVersionString() string {
	if !p.Available {
		return "Not available"
	}
	return p.Version
}

// GetSourceString returns a user-friendly source description
func (p *PythonInfo) GetSourceString() string {
	switch p.Source {
	case "conda":
		return "Conda/Anaconda"
	case "pyenv":
		return "pyenv"
	case "virtualenv":
		return "Virtual Environment"
	case "homebrew":
		return "Homebrew"
	case "system":
		return "System Python"
	case "custom":
		return "Custom Installation"
	default:
		return "Unknown"
	}
}
