package factory

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTranslatePodStatus(t *testing.T) {
	cases := []struct {
		phase corev1.PodPhase
		want  MachineStatus
	}{
		{corev1.PodPending, MachineStatusPending},
		{corev1.PodRunning, MachineStatusRunning},
		{corev1.PodFailed, MachineStatusFailed},
		{corev1.PodSucceeded, MachineStatusPending},
	}
	for _, tc := range cases {
		if got := TranslatePodStatus(tc.phase); got != tc.want {
			t.Errorf("TranslatePodStatus(%v) = %v, want %v", tc.phase, got, tc.want)
		}
	}
}

func TestPodEventToFactoryEvent(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "podustrial-machine-1"},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	ev := PodEventToFactoryEvent(EventMachineAdded, pod)
	if ev.Type != EventMachineAdded {
		t.Errorf("Type = %v, want %v", ev.Type, EventMachineAdded)
	}
	if ev.Machine == nil || ev.Machine.ID != "podustrial-machine-1" || ev.Machine.Status != MachineStatusRunning {
		t.Errorf("Machine = %+v, want ID=podustrial-machine-1 Status=running", ev.Machine)
	}
}

func TestNewClusterStatusEvent(t *testing.T) {
	ev := NewClusterStatusEvent(ClusterStatusDown)
	if ev.Type != EventClusterStatus || ev.ClusterStatus != ClusterStatusDown {
		t.Errorf("got %+v, want Type=cluster_status ClusterStatus=stromausfall", ev)
	}
	if ev.Machine != nil {
		t.Errorf("Machine should be nil for a cluster status event, got %+v", ev.Machine)
	}
}
