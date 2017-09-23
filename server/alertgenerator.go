package server

import (
	"github.com/prometheus/common/model"
)

// GenerateAlert creates alert for service scaler
func GenerateAlert(alertname string,
	serviceName string, status string,
	message string, request string) *model.Alert {
	return &model.Alert{
		Labels: model.LabelSet{
			"alertname": model.LabelValue(alertname),
			"service":   model.LabelValue(serviceName),
			"status":    model.LabelValue(status),
		},
		Annotations: model.LabelSet{
			"message": model.LabelValue(message),
			"request": model.LabelValue(request),
		},
		GeneratorURL: "",
	}
}
