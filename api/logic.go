package api

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

type PodData struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Ip          string `json:"ip"`
	NodeName    string `json:"nodeName"`
	Status      string `json:"status"`
	Age         string `json:"age"`
	UseGpu      bool   `json:"useGpu"`
	UseGpuCount string `json:"useGpuCount"`
	GpuProduct  string `json:"gpuProduct"`
}

func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}
