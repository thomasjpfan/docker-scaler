package server

import "encoding/json"

type groupLabels struct {
	Service string `json:"service,omitempty"`
	Scale   string `json:"scale,omitempty"`
}

// ScaleServiceRequest is the POST body used to scale services
// It only needs the `groupLabels` value of the Alertmanager POST
// webhook request
type ScaleServiceRequest struct {
	GroupLabels groupLabels `json:"groupLabels,omitempty"`
}

// NewScaleServiceRequestBody returns json body of service-related
// POST request
func NewScaleServiceRequestBody(service, scale string) []byte {
	request := ScaleServiceRequest{
		GroupLabels: groupLabels{
			Service: service,
			Scale:   scale,
		},
	}
	output, _ := json.Marshal(request)
	return output
}
