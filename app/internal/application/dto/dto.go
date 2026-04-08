package dto

import (
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"replikator/internal/domain/entity"
	"replikator/internal/domain/valueobject"
)

type SourceDTO struct {
	Namespace         string   `json:"namespace"`
	Name              string   `json:"name"`
	ResourceType      string   `json:"resourceType"`
	Allowed           bool     `json:"allowed"`
	AllowedNamespaces []string `json:"allowedNamespaces"`
	AutoEnabled       bool     `json:"autoEnabled"`
	AutoNamespaces    []string `json:"autoNamespaces"`
	Version           string   `json:"version"`
}

func NewSourceDTO(source *entity.Source) *SourceDTO {
	return &SourceDTO{
		Namespace:         source.Namespace(),
		Name:              source.Name(),
		ResourceType:      string(source.ResourceType()),
		Allowed:           source.IsAllowed(),
		AllowedNamespaces: source.AllowedNamespaces().Patterns(),
		AutoEnabled:       source.IsAutoEnabled(),
		AutoNamespaces:    source.AutoNamespaces().Patterns(),
		Version:           source.Version(),
	}
}

func (d *SourceDTO) ToAnnotations() map[string]string {
	annotations := make(map[string]string)
	annotations[AnnotationReplicationAllowed] = boolToString(d.Allowed)
	if len(d.AllowedNamespaces) > 0 {
		annotations[AnnotationReplicationAllowedNamespaces] = joinNamespaces(d.AllowedNamespaces)
	}
	annotations[AnnotationReplicationAutoEnabled] = boolToString(d.AutoEnabled)
	if len(d.AutoNamespaces) > 0 {
		annotations[AnnotationReplicationAutoNamespaces] = joinNamespaces(d.AutoNamespaces)
	}
	return annotations
}

type MirrorDTO struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	ResourceType       string `json:"resourceType"`
	SourceID           string `json:"sourceID"`
	SourceNamespace    string `json:"sourceNamespace"`
	SourceName         string `json:"sourceName"`
	Version            string `json:"version"`
	ReflectsAnnotation string `json:"reflectsAnnotation"`
	IsAutoCreated      bool   `json:"isAutoCreated"`
	ReflectedAt        string `json:"reflectedAt"`
}

func NewMirrorDTO(mirror *entity.Mirror) *MirrorDTO {
	return &MirrorDTO{
		Namespace:          mirror.Namespace(),
		Name:               mirror.Name(),
		ResourceType:       string(mirror.ResourceType()),
		SourceID:           mirror.SourceID().String(),
		SourceNamespace:    mirror.SourceID().Namespace(),
		SourceName:         mirror.SourceID().Name(),
		Version:            mirror.Version(),
		ReflectsAnnotation: mirror.SourceID().String(),
		IsAutoCreated:      mirror.IsAutoCreated(),
		ReflectedAt:        mirror.ReflectedAt(),
	}
}

func (d *MirrorDTO) ToAnnotations() map[string]string {
	annotations := make(map[string]string)
	annotations[AnnotationReflects] = d.ReflectsAnnotation
	annotations[AnnotationAutoReflects] = boolToString(d.IsAutoCreated)
	if d.ReflectedAt != "" {
		annotations[AnnotationReflectedAt] = d.ReflectedAt
	}
	return annotations
}

type ReflectionRequest struct {
	Source          *entity.Source
	Mirror          *entity.Mirror
	SourceNamespace string
	SourceName      string
	TargetNamespace string
	TargetName      string
}

type ReflectionResult struct {
	SourceID      valueobject.SourceID
	MirrorID      valueobject.MirrorID
	Success       bool
	Error         error
	SourceVersion string
}

func NewReflectionResult(sourceID valueobject.SourceID, mirrorID valueobject.MirrorID, success bool, sourceVersion string) *ReflectionResult {
	return &ReflectionResult{
		SourceID:      sourceID,
		MirrorID:      mirrorID,
		Success:       success,
		SourceVersion: sourceVersion,
	}
}

const (
	AnnotationReplicationAllowed           = "replikator.cloudresty.io/replicate"
	AnnotationReplicationAllowedNamespaces = "replikator.cloudresty.io/replicate-to"
	AnnotationReplicationAutoEnabled       = "replikator.cloudresty.io/auto-replicate"
	AnnotationReplicationAutoNamespaces    = "replikator.cloudresty.io/auto-replicate-to"
	AnnotationReflects                     = "replikator.cloudresty.io/replicated-from"
	AnnotationReflectedVersion             = "replikator.cloudresty.io/replicated-version"
	AnnotationReflectedAt                  = "replikator.cloudresty.io/replicated-at"
	AnnotationAutoReflects                 = "replikator.cloudresty.io/auto-replicated"
	AnnotationSourceResourceVersion        = "replikator.cloudresty.io/source-resource-version"
)

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func joinNamespaces(namespaces []string) string {
	var result strings.Builder
	for i, ns := range namespaces {
		if i > 0 {
			result.WriteString(",")
		}
		result.WriteString(ns)
	}
	return result.String()
}

func ParseSourceAnnotations(obj metav1.Object) (*SourceDTO, error) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return &SourceDTO{}, nil
	}

	dto := &SourceDTO{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	if val, ok := annotations[AnnotationReplicationAllowed]; ok {
		dto.Allowed = val == "true"
	}

	if val, ok := annotations[AnnotationReplicationAllowedNamespaces]; ok {
		dto.AllowedNamespaces = parseNamespaceList(val)
	}

	if val, ok := annotations[AnnotationReplicationAutoEnabled]; ok {
		dto.AutoEnabled = val == "true"
	}

	if val, ok := annotations[AnnotationReplicationAutoNamespaces]; ok {
		dto.AutoNamespaces = parseNamespaceList(val)
	}

	return dto, nil
}

func ParseMirrorAnnotations(obj metav1.Object) (*MirrorDTO, error) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil, nil
	}

	reflects, ok := annotations[AnnotationReflects]
	if !ok || reflects == "" {
		return nil, nil
	}

	parts := splitReflectsAnnotation(reflects)
	if len(parts) != 2 {
		return nil, &ErrInvalidReflectsAnnotation{}
	}

	isAutoReflects := annotations[AnnotationAutoReflects] == "true"
	reflectedAt := annotations[AnnotationReflectedAt]

	return &MirrorDTO{
		Namespace:          obj.GetNamespace(),
		Name:               obj.GetName(),
		ReflectsAnnotation: reflects,
		SourceNamespace:    parts[0],
		SourceName:         parts[1],
		IsAutoCreated:      isAutoReflects,
		ReflectedAt:        reflectedAt,
	}, nil
}

func splitReflectsAnnotation(value string) []string {
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '/' {
			return []string{value[:i], value[i+1:]}
		}
	}
	return []string{"", value}
}

func parseNamespaceList(value string) []string {
	if value == "" {
		return nil
	}
	parts := make([]string, 0)
	for _, p := range splitNamespaces(value) {
		p = trimSpace(p)
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}

func splitNamespaces(value string) []string {
	result := make([]string, 0)
	current := ""
	for _, c := range value {
		if c == ',' {
			if current != "" {
				result = append(result, current)
			}
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

type ErrInvalidReflectsAnnotation struct{}

func (e *ErrInvalidReflectsAnnotation) Error() string {
	return "invalid reflects annotation format, expected 'namespace/name'"
}

func GetResourceVersion(obj interface{}) string {
	switch v := obj.(type) {
	case *corev1.Secret:
		return v.ResourceVersion
	case *corev1.ConfigMap:
		return v.ResourceVersion
	case corev1.Secret:
		return v.ResourceVersion
	case corev1.ConfigMap:
		return v.ResourceVersion
	}
	return ""
}

func IsSecret(obj interface{}) bool {
	_, ok := obj.(*corev1.Secret)
	return ok
}

func IsConfigMap(obj interface{}) bool {
	_, ok := obj.(*corev1.ConfigMap)
	return ok
}

func IsHelmSecret(secret *corev1.Secret) bool {
	if strings.HasPrefix(string(secret.Type), "helm.sh") {
		return true
	}
	if secret.Annotations != nil {
		if _, ok := secret.Annotations["meta.helm.sh/release-name"]; ok {
			return true
		}
		if _, ok := secret.Annotations["helm.sh/release"]; ok {
			return true
		}
	}
	return false
}

func IsHelmResource(obj interface{}) bool {
	switch v := obj.(type) {
	case *corev1.Secret:
		return IsHelmSecret(v)
	case *corev1.ConfigMap:
		return IsHelmConfigMap(v)
	}
	return false
}

func IsHelmConfigMap(cm *corev1.ConfigMap) bool {
	if cm.Annotations != nil {
		if _, ok := cm.Annotations["meta.helm.sh/release-name"]; ok {
			return true
		}
		if _, ok := cm.Annotations["helm.sh/release"]; ok {
			return true
		}
	}
	return false
}

func FormatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func ParseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
