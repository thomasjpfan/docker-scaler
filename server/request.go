package server

type groupLabels struct {
	Service string `json:"service,omitempty"`
	Scale   string `json:"scale,omitempty"`
	By      uint64 `json:"by,omitempty"`
}

// ScaleRequest is the POST body used to scale services/nodes
// It only needs the `groupLabels` value of the Alertmanager POST
// webhook request
type ScaleRequest struct {
	GroupLabels groupLabels `json:"groupLabels,omitempty"`
}
