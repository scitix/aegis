package prom

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/common/model"
)

type Event struct {
	TimeStamps              string `json:"timestamps"`
	Source                  string `json:"source"`
	InvolvedObjectKkind     string `json:"involvedObjectKind"`
	InvolvedObjectNamespace string `json:"involvedObjectNamespace"`
	InvolvedObjectName      string `json:"involvedObjectName"`
	Type                    string `json:"type"`
	Message                 string `json:"message"`
	Reason                  string `json:"reason"`
	Count                   int    `json:"count"`
}

func (p *PromAPI) GetEvent(ctx context.Context, objectkind, objectnamespace, objectname, tpe string) ([]Event, error) {
	labels := make([]string, 0)
	if objectkind != "" {
		labels = append(labels, fmt.Sprintf("involved_object_kind=\"%s\"", objectkind))
	}
	if objectnamespace != "" {
		labels = append(labels, fmt.Sprintf("involved_object_namespace=\"%s\"", objectnamespace))
	}
	if objectname != "" {
		labels = append(labels, fmt.Sprintf("involved_object_name=\"%s\"", objectname))
	}
	if tpe != "" {
		labels = append(labels, fmt.Sprintf("type=\"%s\"", tpe))
	}

	query := fmt.Sprintf("kube_event_count{%s}", strings.Join(labels, ","))

	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveEvent(value), nil
}

func (p *PromAPI) GetEventWithRange(ctx context.Context, objectkind, objectnamespace, objectname, tpe string, duration string) ([]Event, error) {
	labels := make([]string, 0)
	if objectkind != "" {
		labels = append(labels, fmt.Sprintf("involved_object_kind=\"%s\"", objectkind))
	}
	if objectnamespace != "" {
		labels = append(labels, fmt.Sprintf("involved_object_namespace=\"%s\"", objectnamespace))
	}
	if objectname != "" {
		labels = append(labels, fmt.Sprintf("involved_object_name=\"%s\"", objectname))
	}
	if tpe != "" {
		labels = append(labels, fmt.Sprintf("type=\"%s\"", tpe))
	}

	query := fmt.Sprintf("count by (source, involved_object_kind, involved_object_name, involved_object_namespace, reason, message, timestamps, type) (last_over_time(kube_event_count{%s}[%s]))", strings.Join(labels, ","), duration)
	value, err := p.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return resolveEventWithRange(value), nil
}

func resolveEvent(value model.Value) []Event {
	events := make([]Event, 0)

	switch value.Type() {
	case model.ValVector:
		for _, val := range value.(model.Vector) {
			metric := val.Metric
			source := string(metric["source"])
			kind := string(metric["involved_object_kind"])
			name := string(metric["involved_object_name"])
			namespace := string(metric["involved_object_namespace"])
			reason := string(metric["reason"])
			message := string(metric["message"])
			timestamps := string(metric["timestamps"])
			tpe := string(metric["type"])
			count := int(val.Value)

			events = append(events, Event{
				TimeStamps:              timestamps,
				Source:                  source,
				InvolvedObjectKkind:     kind,
				InvolvedObjectName:      name,
				InvolvedObjectNamespace: namespace,
				Type:                    tpe,
				Reason:                  reason,
				Message:                 message,
				Count:                   count,
			})
		}
	default:
		return nil
	}

	sort.Sort(eventList(events))

	return events
}

func resolveEventWithRange(value model.Value) []Event {
	events := make([]Event, 0)
	m := make(map[string]Event)

	switch value.Type() {
	case model.ValVector:
		for _, val := range value.(model.Vector) {
			metric := val.Metric
			source := string(metric["source"])
			kind := string(metric["involved_object_kind"])
			name := string(metric["involved_object_name"])
			namespace := string(metric["involved_object_namespace"])
			reason := string(metric["reason"])
			message := string(metric["message"])
			timestamps := string(metric["timestamps"])
			tpe := string(metric["type"])
			count := int(val.Value)

			if val, ok := m[message]; !ok {
				m[message] = Event{
					TimeStamps:              timestamps,
					Source:                  source,
					InvolvedObjectKkind:     kind,
					InvolvedObjectName:      name,
					InvolvedObjectNamespace: namespace,
					Type:                    tpe,
					Reason:                  reason,
					Message:                 message,
					Count:                   count,
				}
			} else {
				m[message] = Event{
					TimeStamps:              maxString(val.TimeStamps, timestamps),
					Source:                  source,
					InvolvedObjectKkind:     kind,
					InvolvedObjectName:      name,
					InvolvedObjectNamespace: namespace,
					Type:                    tpe,
					Reason:                  reason,
					Message:                 message,
					Count:                   val.Count + count,
				}
			}
		}
	default:
		return nil
	}

	for _, event := range m {
		events = append(events, event)
	}

	sort.Sort(eventList(events))

	return events
}


func maxString(a, b string) string {
	if strings.Compare(a, b) >= 0 {
		return a
	} else {
		return b
	}
}

type eventList []Event

func (el eventList) Len() int {
	return len(el)
}

func (el eventList) Less(i, j int) bool {
	return strings.Compare(el[i].TimeStamps, el[j].TimeStamps) > 0
}

func (el eventList) Swap(i, j int) {
	el[i], el[j] = el[j], el[i]
}
