package common

import (
	v1 "k8s.io/api/core/v1"
	"strconv"
)

const (
	EasyAuthAnnotation = "linkerd-io/easyauth-enabled"
)

func IsEasyAuthEnabled(pod *v1.Pod) bool {
	valStr := pod.GetAnnotations()[EasyAuthAnnotation]
	if valStr != "" {
		valBool, err := strconv.ParseBool(valStr)
		if err == nil && valBool {
			return true
		}
	}
	return false
}
