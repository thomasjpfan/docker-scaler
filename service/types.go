package service

import (
	"github.com/prometheus/common/model"
)

type alertmanagerAlertResponse struct {
	Status    string        `json:"status"`
	Data      []*alertGroup `json:"data,omitempty"`
	ErrorType string        `json:"errorType,omitempty"`
	Error     string        `json:"error,omitempty"`
}

type alertGroup struct {
	Labels   model.LabelSet `json:"labels"`
	GroupKey string         `json:"groupKey"`
	Blocks   []*alertBlock  `json:"blocks"`
}

// AlertStatus stores the state and values associated with an Alert.
type AlertStatus struct {
	State       string   `json:"state"`
	SilencedBy  []string `json:"silencedBy"`
	InhibitedBy []string `json:"inhibitedBy"`
}

// APIAlert are alerts from alertmanager
type APIAlert struct {
	*model.Alert
	Status AlertStatus `json:"status"`
}

type alertBlock struct {
	RouteOpts interface{} `json:"routeOpts"`
	Alerts    []*APIAlert `json:"alerts"`
}
