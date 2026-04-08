package annotations

import (
	"time"

	"replikator/internal/application/dto"
)

func MirrorAnnotations(sourceID, version string) map[string]string {
	return map[string]string{
		dto.AnnotationReflects:         sourceID,
		dto.AnnotationReflectedVersion: version,
		dto.AnnotationReflectedAt:      time.Now().UTC().Format(time.RFC3339),
	}
}

func AutoMirrorAnnotations(sourceID, version string) map[string]string {
	return map[string]string{
		dto.AnnotationReflects:         sourceID,
		dto.AnnotationReflectedVersion: version,
		dto.AnnotationReflectedAt:      time.Now().UTC().Format(time.RFC3339),
		dto.AnnotationAutoReflects:     "true",
	}
}

func SourceAnnotations(allowed bool, allowedNamespaces string, autoEnabled bool, autoNamespaces string) map[string]string {
	annotations := map[string]string{
		dto.AnnotationReplicationAllowed: boolToString(allowed),
	}

	if allowedNamespaces != "" {
		annotations[dto.AnnotationReplicationAllowedNamespaces] = allowedNamespaces
	}

	annotations[dto.AnnotationReplicationAutoEnabled] = boolToString(autoEnabled)

	if autoNamespaces != "" {
		annotations[dto.AnnotationReplicationAutoNamespaces] = autoNamespaces
	}

	return annotations
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
