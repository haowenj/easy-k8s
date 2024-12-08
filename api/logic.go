package api

import (
	"fmt"
	"time"
)

type PodData struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Ip          string `json:"ip"`
	NodeName    string `json:"nodeName"`
	Status      string `json:"status"`
	Age         string `json:"age"`
	UseGpu      bool   `json:"useGpu"`
	UseGpuCount int    `json:"useGpuCount"`
	GpuProduct  string `json:"gpuProduct"`
}

func calculateAge(creationTime time.Time) string {
	now := time.Now()
	duration := now.Sub(creationTime)

	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	if days > 0 {
		if days > 5 {
			return fmt.Sprintf("%dd", days)
		}
		return fmt.Sprintf("%dd%dh", days, hours)
	} else if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%d秒", seconds)
	}
}
