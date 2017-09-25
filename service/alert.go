package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
)

// AlertServicer interface to send alerts
type AlertServicer interface {
	Send(alertName string, serviceName string,
		request string, status string,
		message string) error
}

// SilentAlertService is a stub for an alert service
type SilentAlertService struct{}

// Send is a stub for an alert service
func (s SilentAlertService) Send(alertName string,
	serviceName string, request string,
	status string, message string) error {
	return nil
}

// AlertService sends alerts to an alertmanager
type AlertService struct {
	url string
}

// NewAlertService creates new AlertService
func NewAlertService(url string) *AlertService {
	return &AlertService{
		url: url,
	}
}

// Send sends alert to alertmanager
func (a AlertService) Send(alertName string, serviceName string, request string, status string, message string) error {
	alert := generateAlert(alertName, serviceName, request, status, message)
	alerts := []*model.Alert{alert}
	alertsJSON, _ := json.Marshal(alerts)
	r := bytes.NewReader(alertsJSON)

	endpoint := fmt.Sprintf("%s/api/v1/alerts", a.url)
	resp, err := http.Post(endpoint, "application/json", r)
	if err != nil {
		return errors.Wrap(err, "Failed to send alert to alertmanager")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "Unable to read body of alert response")
	}

	var resJSON alertmanagerAlertResponse
	err = json.Unmarshal(body, &resJSON)
	if err != nil {
		return errors.Wrap(err, "Unable to parse alert response")
	}
	if resJSON.Status != "success" {
		return fmt.Errorf("Send request to alertmanager failed")
	}
	return nil
}

func generateAlert(alertName string, serviceName string,
	request string, status string, summary string) *model.Alert {
	return &model.Alert{
		Labels: model.LabelSet{
			"alertname": model.LabelValue(alertName),
			"service":   model.LabelValue(serviceName),
			"status":    model.LabelValue(status),
		},
		Annotations: model.LabelSet{
			"summary": model.LabelValue(summary),
			"request": model.LabelValue(request),
		},
		GeneratorURL: "",
	}
}

// FetchAlerts gets alerts from alertmanager
// https://github.com/prometheus/alertmanager/blob/5aff15b30fd10459b9ebf0ef754e1794b9ffd1ff/cli/alert.go#L86
// Use for testing purposes only
func FetchAlerts(path, alertname, status, service string) ([]*APIAlert, error) {
	alertResponse := alertmanagerAlertResponse{}
	endpoint := fmt.Sprintf("%s/api/v1/alerts/groups?filter=alertname=%s,status=%s,service=%s", path, alertname, status, service)
	res, err := http.Get(endpoint)
	if err != nil {
		return []*APIAlert{}, err
	}
	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&alertResponse)
	if err != nil {
		return []*APIAlert{}, fmt.Errorf("Unable to decode json response: %s", err)
	}

	if alertResponse.Status != "success" {
		return []*APIAlert{}, fmt.Errorf("[%s] %s", alertResponse.ErrorType, alertResponse.Error)
	}

	return flattenAlertOverview(alertResponse.Data), nil
}

func flattenAlertOverview(overview []*alertGroup) []*APIAlert {
	alerts := []*APIAlert{}
	for _, group := range overview {
		for _, block := range group.Blocks {
			alerts = append(alerts, block.Alerts...)
		}
	}
	return alerts
}
