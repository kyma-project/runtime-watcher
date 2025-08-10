package types

// ObjectKey represents a unique identifier for a Kubernetes object
type ObjectKey struct {
	// Namespace is the namespace of the object. For cluster-scoped objects, this is empty.
	Namespace string `json:"namespace,omitempty"`
	// Name is the name of the object
	Name string `json:"name"`
}

// String returns the general purpose string representation
func (k ObjectKey) String() string {
	if k.Namespace == "" {
		return k.Name
	}
	return k.Namespace + "/" + k.Name
}
