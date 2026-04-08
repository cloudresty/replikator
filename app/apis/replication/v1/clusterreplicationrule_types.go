package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ReplicationRuleSpec struct {
	SourceNamespace    string           `json:"sourceNamespace"`
	SourceSelector     string           `json:"sourceSelector"`
	ResourceTypes      []string         `json:"resourceTypes,omitempty"`
	TargetNamespaces   TargetNamespaces `json:"targetNamespaces"`
	TargetNameTemplate string           `json:"targetNameTemplate,omitempty"`
	AnnotationsToCopy  []string         `json:"annotationsToCopy,omitempty"`
	LabelsToCopy       []string         `json:"labelsToCopy,omitempty"`
}

type TargetNamespaces struct {
	Selector string   `json:"selector,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
	Includes []string `json:"includes,omitempty"`
}

type ReplicationRuleStatus struct {
	ActiveMirrors      int32                     `json:"activeMirrors"`
	Errors             int32                     `json:"errors"`
	LastSyncTime       *metav1.Time              `json:"lastSyncTime,omitempty"`
	ObservedGeneration int64                     `json:"observedGeneration"`
	SyncedResources    []SyncedResource          `json:"syncedResources,omitempty"`
	MirroredCount      int32                     `json:"mirroredCount"`
	Condition          *ReplicationRuleCondition `json:"condition,omitempty"`
}

type SyncedResource struct {
	Name            string       `json:"name"`
	Namespace       string       `json:"namespace"`
	ResourceType    string       `json:"resourceType"`
	MirroredToCount int          `json:"mirroredToCount"`
	LastSyncedTime  *metav1.Time `json:"lastSyncedTime,omitempty"`
}

type ReplicationRuleConditionType string

const (
	ReplicationRuleActive ReplicationRuleConditionType = "Active"
	ReplicationRuleError  ReplicationRuleConditionType = "Error"
)

type ReplicationRuleCondition struct {
	Type               ReplicationRuleConditionType `json:"type"`
	Status             metav1.ConditionStatus       `json:"status"`
	LastTransitionTime metav1.Time                  `json:"lastTransitionTime,omitempty"`
	Reason             string                       `json:"reason,omitempty"`
	Message            string                       `json:"message,omitempty"`
}

type ClusterReplicationRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ReplicationRuleSpec   `json:"spec,omitempty"`
	Status ReplicationRuleStatus `json:"status,omitempty"`
}

type ClusterReplicationRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterReplicationRule `json:"items"`
}

func (c *ClusterReplicationRule) DeepCopyInto(out *ClusterReplicationRule) {
	*out = *c
	out.Spec = ReplicationRuleSpec{
		SourceNamespace:    c.Spec.SourceNamespace,
		SourceSelector:     c.Spec.SourceSelector,
		TargetNameTemplate: c.Spec.TargetNameTemplate,
	}
	if c.Spec.ResourceTypes != nil {
		out.Spec.ResourceTypes = make([]string, len(c.Spec.ResourceTypes))
		copy(out.Spec.ResourceTypes, c.Spec.ResourceTypes)
	}
	out.Spec.TargetNamespaces = TargetNamespaces{
		Selector: c.Spec.TargetNamespaces.Selector,
	}
	if c.Spec.TargetNamespaces.Excludes != nil {
		out.Spec.TargetNamespaces.Excludes = make([]string, len(c.Spec.TargetNamespaces.Excludes))
		copy(out.Spec.TargetNamespaces.Excludes, c.Spec.TargetNamespaces.Excludes)
	}
	if c.Spec.TargetNamespaces.Includes != nil {
		out.Spec.TargetNamespaces.Includes = make([]string, len(c.Spec.TargetNamespaces.Includes))
		copy(out.Spec.TargetNamespaces.Includes, c.Spec.TargetNamespaces.Includes)
	}
	if c.Spec.AnnotationsToCopy != nil {
		out.Spec.AnnotationsToCopy = make([]string, len(c.Spec.AnnotationsToCopy))
		copy(out.Spec.AnnotationsToCopy, c.Spec.AnnotationsToCopy)
	}
	if c.Spec.LabelsToCopy != nil {
		out.Spec.LabelsToCopy = make([]string, len(c.Spec.LabelsToCopy))
		copy(out.Spec.LabelsToCopy, c.Spec.LabelsToCopy)
	}
	if c.Status.SyncedResources != nil {
		out.Status.SyncedResources = make([]SyncedResource, len(c.Status.SyncedResources))
		copy(out.Status.SyncedResources, c.Status.SyncedResources)
	}
	if c.Status.LastSyncTime != nil {
		out.Status.LastSyncTime = &metav1.Time{Time: c.Status.LastSyncTime.Time}
	}
	if c.Status.Condition != nil {
		out.Status.Condition = &ReplicationRuleCondition{
			Type:               c.Status.Condition.Type,
			Status:             c.Status.Condition.Status,
			LastTransitionTime: c.Status.Condition.LastTransitionTime,
			Reason:             c.Status.Condition.Reason,
			Message:            c.Status.Condition.Message,
		}
	}
}

func (c *ClusterReplicationRule) DeepCopyObject() runtime.Object {
	out := &ClusterReplicationRule{}
	c.DeepCopyInto(out)
	return out
}

func (c *ClusterReplicationRuleList) DeepCopyInto(out *ClusterReplicationRuleList) {
	*out = *c
	if c.Items != nil {
		out.Items = make([]ClusterReplicationRule, len(c.Items))
		for i := range c.Items {
			c.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (c *ClusterReplicationRuleList) DeepCopyObject() runtime.Object {
	out := &ClusterReplicationRuleList{}
	c.DeepCopyInto(out)
	return out
}
