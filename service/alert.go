package service

import (
	"github.com/prometheus/common/model"
)

// AlertServicer interface to send alerts
type AlertServicer interface {
	Send(alertName string, serviceName string,
		request string, status string,
		message string) error
}

func generateAlert(alertName string, serviceName string,
	request string, status string, message string) *model.Alert {
	return &model.Alert{
		Labels: model.LabelSet{
			"alertname": model.LabelValue(alertName),
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

// SilentAlertService is a stub for an alert service
type SilentAlertService struct{}

// Send is a stub for an alert service
func (s SilentAlertService) Send(alertName string,
	serviceName string, request string,
	status string, message string) error {
	return nil
}
