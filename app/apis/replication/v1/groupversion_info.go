package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: "replikator.cloudresty.io", Version: "v1"}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	addKnownTypes      = func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion,
			&ClusterReplicationRule{},
			&ClusterReplicationRuleList{},
		)
		// Register standard meta types (ListOptions, DeleteOptions, etc.)
		// under this group version so that client-go informers/reflectors
		// can properly convert list/watch requests.
		metav1.AddToGroupVersion(s, SchemeGroupVersion)
		return nil
	}
)

func AddToScheme(s *runtime.Scheme) error {
	return SchemeBuilder.AddToScheme(s)
}

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
