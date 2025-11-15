package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

// ModuleInfo holds information about Environment Modules (Lmod/TCL)
type ModuleInfo struct {
	Available bool
	Type      string // "lmod", "tcl", "unknown"
	Version   string
}

// CheckModules checks if Environment Modules system is available
func CheckModules() (*ModuleInfo, error) {
	info := &ModuleInfo{
		Available: false,
		Type:      "unknown",
	}

	// Try to execute 'module avail' command
	// Note: 'module' is typically a shell function, so we need to source it first
	// We'll try different approaches to detect it

	// Method 1: Try direct module command (works if module is in PATH as script)
	cmd := exec.Command("bash", "-c", "module avail 2>&1")
	output, err := cmd.CombinedOutput()

	if err == nil && len(output) > 0 {
		info.Available = true
		info.Type = determineModuleType(string(output))
		return info, nil
	}

	// Method 2: Check if MODULESHOME is set (Lmod)
	cmd = exec.Command("bash", "-c", "echo $MODULESHOME")
	output, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		info.Available = true
		info.Type = "lmod"

		// Try to get Lmod version
		versionCmd := exec.Command("bash", "-c", "module --version 2>&1")
		versionOutput, err := versionCmd.CombinedOutput()
		if err == nil {
			info.Version = parseModuleVersion(string(versionOutput))
		}
		return info, nil
	}

	// Method 3: Check for Lmod explicitly
	cmd = exec.Command("bash", "-c", "command -v lmod")
	output, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		info.Available = true
		info.Type = "lmod"
		return info, nil
	}

	// Method 4: Check for TCL modules
	cmd = exec.Command("bash", "-c", "command -v modulecmd")
	output, err = cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		info.Available = true
		info.Type = "tcl"
		return info, nil
	}

	// Method 5: Source common module init files and try
	commonInitPaths := []string{
		"/usr/share/lmod/lmod/init/bash",
		"/etc/profile.d/modules.sh",
		"/usr/share/Modules/init/bash",
	}

	for _, initPath := range commonInitPaths {
		cmd = exec.Command("bash", "-c", fmt.Sprintf("source %s 2>/dev/null && module avail 2>&1", initPath))
		output, err = cmd.CombinedOutput()
		if err == nil && len(output) > 0 {
			info.Available = true
			info.Type = determineModuleType(string(output))
			return info, nil
		}
	}

	return info, nil
}

// determineModuleType tries to identify if it's Lmod or TCL modules
func determineModuleType(output string) string {
	outputLower := strings.ToLower(output)

	if strings.Contains(outputLower, "lmod") {
		return "lmod"
	}

	if strings.Contains(outputLower, "modules based on lua") {
		return "lmod"
	}

	if strings.Contains(outputLower, "tcl") {
		return "tcl"
	}

	// Default to lmod as it's more common in HPC environments
	return "lmod"
}

// parseModuleVersion extracts version from module --version output
func parseModuleVersion(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(strings.ToLower(line), "version") {
			// Extract version number
			parts := strings.Fields(line)
			for i, part := range parts {
				if strings.Contains(strings.ToLower(part), "version") && i+1 < len(parts) {
					return parts[i+1]
				}
			}
		}
	}
	return ""
}

// GetTypeString returns a user-friendly type description
func (m *ModuleInfo) GetTypeString() string {
	if !m.Available {
		return "Not available"
	}

	switch m.Type {
	case "lmod":
		return "Lmod (Lua-based)"
	case "tcl":
		return "Environment Modules (TCL)"
	default:
		return "Environment Modules"
	}
}

// GetVersionString returns version or status
func (m *ModuleInfo) GetVersionString() string {
	if !m.Available {
		return "Not installed"
	}
	if m.Version != "" {
		return m.Version
	}
	return "Unknown version"
}
