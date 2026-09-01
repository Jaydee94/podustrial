package factory

import (
	corev1 "k8s.io/api/core/v1"
)

type MachineStatus string

const (
	MachineStatusPending MachineStatus = "pending"
	MachineStatusRunning MachineStatus = "running"
	MachineStatusFailed  MachineStatus = "failed"
)

type Machine struct {
	ID     string        `json:"id"`
	Status MachineStatus `json:"status"`
}

type ClusterStatus string

const (
	ClusterStatusOK   ClusterStatus = "ok"
	ClusterStatusDown ClusterStatus = "stromausfall"
)

type EventType string

const (
	EventMachineAdded   EventType = "machine_added"
	EventMachineUpdated EventType = "machine_updated"
	EventMachineRemoved EventType = "machine_removed"
	EventClusterStatus  EventType = "cluster_status"
)

type Event struct {
	Type          EventType     `json:"type"`
	Machine       *Machine      `json:"machine,omitempty"`
	ClusterStatus ClusterStatus `json:"clusterStatus,omitempty"`
}

func TranslatePodStatus(phase corev1.PodPhase) MachineStatus {
	switch phase {
	case corev1.PodRunning:
		return MachineStatusRunning
	case corev1.PodFailed:
		return MachineStatusFailed
	default:
		return MachineStatusPending
	}
}

func PodEventToFactoryEvent(eventType EventType, pod *corev1.Pod) Event {
	return Event{
		Type: eventType,
		Machine: &Machine{
			ID:     pod.Name,
			Status: TranslatePodStatus(pod.Status.Phase),
		},
	}
}

func NewClusterStatusEvent(status ClusterStatus) Event {
	return Event{Type: EventClusterStatus, ClusterStatus: status}
}
