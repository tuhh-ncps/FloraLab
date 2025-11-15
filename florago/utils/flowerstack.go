package utils

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// FlowerStackState holds the state of the Flower-AI stack deployment
type FlowerStackState struct {
	mu             sync.RWMutex
	JobID          string                       `json:"job_id"`
	Status         string                       `json:"status"` // pending, starting, running, stopping, stopped, failed
	NumNodes       int                          `json:"num_nodes"`
	ServerNode     *FlowerServerNode            `json:"server_node,omitempty"`
	ClientNodes    map[string]*FlowerClientNode `json:"client_nodes"`
	StartTime      time.Time                    `json:"start_time"`
	CompletionTime time.Time                    `json:"completion_time,omitempty"`
	ExpectedNodes  int                          `json:"expected_nodes"` // 1 server + N clients
	CompletedNodes int                          `json:"completed_nodes"`
}

// FlowerServerNode represents the master node with superlink + superexec
type FlowerServerNode struct {
	NodeID             string    `json:"node_id"`
	Hostname           string    `json:"hostname"`
	IP                 string    `json:"ip"`
	SuperlinkAddress   string    `json:"superlink_address"`
	ServerAppIOAPIPort int       `json:"server_app_io_api_port"`
	FleetAPIPort       int       `json:"fleet_api_port"`
	ControlAPIPort     int       `json:"control_api_port"`
	SuperexecAddress   string    `json:"superexec_address"`
	Status             string    `json:"status"` // starting, ready, failed
	StartedAt          time.Time `json:"started_at"`
}

// FlowerClientNode represents a worker node with supernode + superexec
type FlowerClientNode struct {
	NodeID             string    `json:"node_id"`
	Hostname           string    `json:"hostname"`
	IP                 string    `json:"ip"`
	SupernodeAddress   string    `json:"supernode_address"`
	ClientAppIOAPIPort int       `json:"client_app_io_api_port"`
	SuperexecAddress   string    `json:"superexec_address"`
	Status             string    `json:"status"` // waiting, starting, ready, failed
	StartedAt          time.Time `json:"started_at"`
}

// FlowerStackManager manages the Flower-AI stack state
type FlowerStackManager struct {
	mu     sync.RWMutex
	state  *FlowerStackState
	logger *Logger
}

// NewFlowerStackManager creates a new Flower stack manager
func NewFlowerStackManager(logger *Logger) *FlowerStackManager {
	return &FlowerStackManager{
		logger: logger,
	}
}

// InitializeStack initializes a new Flower stack deployment
func (m *FlowerStackManager) InitializeStack(jobID string, numNodes int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state = &FlowerStackState{
		JobID:          jobID,
		Status:         "pending",
		NumNodes:       numNodes,
		ClientNodes:    make(map[string]*FlowerClientNode),
		StartTime:      time.Now(),
		ExpectedNodes:  1 + numNodes, // 1 server + N clients
		CompletedNodes: 0,
	}
}

// RegisterServerNode registers the server node and its services
func (m *FlowerStackManager) RegisterServerNode(node *FlowerServerNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		return fmt.Errorf("stack not initialized")
	}

	m.state.ServerNode = node
	m.state.Status = "starting"

	if node.Status == "ready" {
		m.state.CompletedNodes++
		m.checkCompletion()
	}

	m.logger.Info("Server node registered: %s (IP: %s)", node.NodeID, node.IP)
	return nil
}

// RegisterClientNode registers a client node
func (m *FlowerStackManager) RegisterClientNode(node *FlowerClientNode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		return fmt.Errorf("stack not initialized")
	}

	m.state.ClientNodes[node.NodeID] = node

	if node.Status == "ready" {
		m.state.CompletedNodes++
		m.checkCompletion()
	}

	m.logger.Info("Client node registered: %s (IP: %s)", node.NodeID, node.IP)
	return nil
}

// UpdateServerNodeStatus updates the server node status
func (m *FlowerStackManager) UpdateServerNodeStatus(nodeID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil || m.state.ServerNode == nil {
		return fmt.Errorf("server node not found")
	}

	oldStatus := m.state.ServerNode.Status
	m.state.ServerNode.Status = status

	if oldStatus != "ready" && status == "ready" {
		m.state.CompletedNodes++
		m.checkCompletion()
	}

	return nil
}

// UpdateClientNodeStatus updates a client node status
func (m *FlowerStackManager) UpdateClientNodeStatus(nodeID string, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == nil {
		return fmt.Errorf("stack not initialized")
	}

	node, exists := m.state.ClientNodes[nodeID]
	if !exists {
		return fmt.Errorf("client node not found: %s", nodeID)
	}

	oldStatus := node.Status
	node.Status = status

	if oldStatus != "ready" && status == "ready" {
		m.state.CompletedNodes++
		m.checkCompletion()
	}

	return nil
}

// checkCompletion checks if all nodes are ready (must be called with lock held)
func (m *FlowerStackManager) checkCompletion() {
	if m.state.CompletedNodes >= m.state.ExpectedNodes {
		m.state.Status = "running"
		m.state.CompletionTime = time.Now()
		m.logger.Success("Flower stack fully deployed! All %d nodes are ready", m.state.ExpectedNodes)
	}
}

// GetServerInfo returns the server node information (blocking until ready or timeout)
func (m *FlowerStackManager) GetServerInfo(timeout time.Duration) (*FlowerServerNode, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		m.mu.RLock()
		if m.state != nil && m.state.ServerNode != nil && m.state.ServerNode.Status == "ready" {
			node := m.state.ServerNode
			m.mu.RUnlock()
			return node, nil
		}
		m.mu.RUnlock()

		time.Sleep(500 * time.Millisecond)
	}

	return nil, fmt.Errorf("timeout waiting for server node to be ready")
}

// GetState returns the current stack state
func (m *FlowerStackManager) GetState() *FlowerStackState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.state == nil {
		return nil
	}

	// Create a copy to avoid race conditions
	stateCopy := &FlowerStackState{
		JobID:          m.state.JobID,
		Status:         m.state.Status,
		NumNodes:       m.state.NumNodes,
		StartTime:      m.state.StartTime,
		CompletionTime: m.state.CompletionTime,
		ExpectedNodes:  m.state.ExpectedNodes,
		CompletedNodes: m.state.CompletedNodes,
		ClientNodes:    make(map[string]*FlowerClientNode),
	}

	if m.state.ServerNode != nil {
		serverCopy := *m.state.ServerNode
		stateCopy.ServerNode = &serverCopy
	}

	for k, v := range m.state.ClientNodes {
		clientCopy := *v
		stateCopy.ClientNodes[k] = &clientCopy
	}

	return stateCopy
}

// ClearState clears the current stack state
func (m *FlowerStackManager) ClearState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = nil
	m.logger.Info("Flower stack state cleared")
}

// IsStackRunning checks if a stack is currently running
func (m *FlowerStackManager) IsStackRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state != nil && (m.state.Status == "starting" || m.state.Status == "running")
}

// ToJSON converts the state to JSON
func (s *FlowerStackState) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
