package server

import "encoding/json"

type groupLabels struct {
	Service string `json:"service,omitempty"`
	Scale   string `json:"scale,omitempty"`
}

// ScaleRequest is the POST body used to scale services/nodes
// It only needs the `groupLabels` value of the Alertmanager POST
// webhook request
type ScaleRequest struct {
	GroupLabels groupLabels `json:"groupLabels,omitempty"`
}

// NewScaleRequestBody returns json body of service-related
// POST request
func NewScaleRequestBody(service, scale string) []byte {
	request := ScaleRequest{
		GroupLabels: groupLabels{
			Service: service,
			Scale:   scale,
		},
	}
	output, _ := json.Marshal(request)
	return output
}
