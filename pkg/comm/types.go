package comm

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// error
var (
	NodeNotFoundErr = errors.New("node not found")
	PodNotFoundErr  = errors.New("pod not found")
)

const (
	NodeLabelRole       = "kubernetes.io/role"
	LabelNodeRolePrefix = "node-role.kubernetes.io/"
	LabelCustomPrefix   = "osgalaxy.io"
)

var DecodeLables = map[string]struct{}{
	"osgalaxy.io/city":     {},
	"osgalaxy.io/country":  {},
	"osgalaxy.io/province": {},
}

var NodeGVR = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "nodes",
}

// PatchOperation dynamicClient patch request JSONPatchType
type PatchOperation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}
