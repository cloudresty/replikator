package v1

import (
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
		return nil
	}
)

func AddToScheme(s *runtime.Scheme) error {
	return SchemeBuilder.AddToScheme(s)
}

func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}
