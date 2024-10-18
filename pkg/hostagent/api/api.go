package api

type Info struct {
	SSHLocalPort int `json:"sshLocalPort,omitempty"`
}

type RunState = string

const (
	StateRunning RunState = "running"
	StatePaused  RunState = "paused"
)

type Status struct {
	State RunState `json:"state"`
}
