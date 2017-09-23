package service

import (
	"github.com/prometheus/common/model"
)

// AlertServicer interface to send alerts
type AlertServicer interface {
	Send(alert *model.Alert) error
}
