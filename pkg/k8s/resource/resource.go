package resource

import (
	corev1 "k8s.io/api/core/v1"
)

func PodRequestsAndLimits(pod *corev1.Pod) (reqs, limits corev1.ResourceList) {
	return podRequests(pod), podLimits(pod)
}

func podRequests(pod *corev1.Pod) corev1.ResourceList {
	// attempt to reuse the maps if passed, or allocate otherwise
	reqs := corev1.ResourceList{}

	containerStatuses := map[string]*corev1.ContainerStatus{}
	for i := range pod.Status.ContainerStatuses {
		containerStatuses[pod.Status.ContainerStatuses[i].Name] = &pod.Status.ContainerStatuses[i]
	}

	for _, container := range pod.Spec.Containers {
		containerReqs := container.Resources.Requests
		cs, found := containerStatuses[container.Name]
		if found && cs.Resources != nil {
			if pod.Status.Resize == corev1.PodResizeStatusInfeasible {
				containerReqs = cs.Resources.Requests.DeepCopy()
			} else {
				containerReqs = getMax(container.Resources.Requests, cs.Resources.Requests)
			}
		}
		addResourceList(reqs, containerReqs)
	}

	restartableInitContainerReqs := corev1.ResourceList{}
	initContainerReqs := corev1.ResourceList{}

	for _, container := range pod.Spec.InitContainers {
		containerReqs := container.Resources.Requests

		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			// and add them to the resulting cumulative container requests
			addResourceList(reqs, containerReqs)

			// track our cumulative restartable init container resources
			addResourceList(restartableInitContainerReqs, containerReqs)
			containerReqs = restartableInitContainerReqs
		} else {
			tmp := corev1.ResourceList{}
			addResourceList(tmp, containerReqs)
			addResourceList(tmp, restartableInitContainerReqs)
			containerReqs = tmp
		}
		maxResourceList(initContainerReqs, containerReqs)
	}

	maxResourceList(reqs, initContainerReqs)

	// Add overhead for running a pod to the sum of requests if requested:
	if pod.Spec.Overhead != nil {
		addResourceList(reqs, pod.Spec.Overhead)
	}

	return reqs
}

// podLimits is a simplified form of PodLimits from k8s.io/kubernetes/pkg/api/v1/resource that doesn't check
// feature gate enablement and avoids adding a dependency on k8s.io/kubernetes/pkg/apis/core/v1 for kubectl.
func podLimits(pod *corev1.Pod) corev1.ResourceList {
	limits := corev1.ResourceList{}

	for _, container := range pod.Spec.Containers {
		addResourceList(limits, container.Resources.Limits)
	}

	restartableInitContainerLimits := corev1.ResourceList{}
	initContainerLimits := corev1.ResourceList{}

	for _, container := range pod.Spec.InitContainers {
		containerLimits := container.Resources.Limits
		// Is the init container marked as a restartable init container?
		if container.RestartPolicy != nil && *container.RestartPolicy == corev1.ContainerRestartPolicyAlways {
			addResourceList(limits, containerLimits)

			// track our cumulative restartable init container resources
			addResourceList(restartableInitContainerLimits, containerLimits)
			containerLimits = restartableInitContainerLimits
		} else {
			tmp := corev1.ResourceList{}
			addResourceList(tmp, containerLimits)
			addResourceList(tmp, restartableInitContainerLimits)
			containerLimits = tmp
		}

		maxResourceList(initContainerLimits, containerLimits)
	}

	maxResourceList(limits, initContainerLimits)

	// Add overhead to non-zero limits if requested:
	if pod.Spec.Overhead != nil {
		for name, quantity := range pod.Spec.Overhead {
			if value, ok := limits[name]; ok && !value.IsZero() {
				value.Add(quantity)
				limits[name] = value
			}
		}
	}

	return limits
}

func getMax(a corev1.ResourceList, b corev1.ResourceList) corev1.ResourceList {
	result := corev1.ResourceList{}
	for key, value := range a {
		if other, found := b[key]; found {
			if value.Cmp(other) <= 0 {
				result[key] = other.DeepCopy()
				continue
			}
		}
		result[key] = value.DeepCopy()
	}
	for key, value := range b {
		if _, found := result[key]; !found {
			result[key] = value.DeepCopy()
		}
	}
	return result
}

// addResourceList adds the resources in newList to list
func addResourceList(list, new corev1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list
func maxResourceList(list, new corev1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = quantity.DeepCopy()
			continue
		} else {
			if quantity.Cmp(value) > 0 {
				list[name] = quantity.DeepCopy()
			}
		}
	}
}
