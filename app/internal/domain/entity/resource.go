package entity

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"replikator/internal/domain/valueobject"
)

type ResourceType string

const (
	ResourceTypeSecret    ResourceType = "Secret"
	ResourceTypeConfigMap ResourceType = "ConfigMap"
)

type ResourceKind interface {
	corev1.Secret | corev1.ConfigMap
}

type Mirror struct {
	id            valueobject.MirrorID
	sourceID      valueobject.SourceID
	namespace     string
	name          string
	resourceType  ResourceType
	version       string
	enabled       bool
	isAutoCreated bool
	reflectedAt   string
}

func NewMirror(id valueobject.MirrorID, sourceID valueobject.SourceID, namespace, name string, resourceType ResourceType) *Mirror {
	return &Mirror{
		id:            id,
		sourceID:      sourceID,
		namespace:     namespace,
		name:          name,
		resourceType:  resourceType,
		enabled:       true,
		isAutoCreated: false,
		reflectedAt:   "",
	}
}

func NewAutoMirror(id valueobject.MirrorID, sourceID valueobject.SourceID, namespace, name string, resourceType ResourceType) *Mirror {
	m := NewMirror(id, sourceID, namespace, name, resourceType)
	m.isAutoCreated = true
	return m
}

func (m *Mirror) ID() valueobject.MirrorID {
	return m.id
}

func (m *Mirror) SourceID() valueobject.SourceID {
	return m.sourceID
}

func (m *Mirror) Namespace() string {
	return m.namespace
}

func (m *Mirror) Name() string {
	return m.name
}

func (m *Mirror) ResourceType() ResourceType {
	return m.resourceType
}

func (m *Mirror) Version() string {
	return m.version
}

func (m *Mirror) SetVersion(v string) {
	m.version = v
}

func (m *Mirror) IsEnabled() bool {
	return m.enabled
}

func (m *Mirror) Disable() {
	m.enabled = false
}

func (m *Mirror) Enable() {
	m.enabled = true
}

func (m *Mirror) IsAutoCreated() bool {
	return m.isAutoCreated
}

func (m *Mirror) SetAutoCreated(auto bool) {
	m.isAutoCreated = auto
}

func (m *Mirror) ReflectedAt() string {
	return m.reflectedAt
}

func (m *Mirror) SetReflectedAt(t string) {
	m.reflectedAt = t
}

func (m *Mirror) SetReflectedAtTime(t time.Time) {
	m.reflectedAt = t.UTC().Format(time.RFC3339)
}

func (m *Mirror) FullName() string {
	return fmt.Sprintf("%s/%s", m.namespace, m.name)
}

func (m *Mirror) ToObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      m.name,
		Namespace: m.namespace,
	}
}

type Source struct {
	id                valueobject.SourceID
	namespace         string
	name              string
	targetName        string
	resourceType      ResourceType
	allowed           bool
	allowedNamespaces valueobject.AllowedNamespaces
	autoEnabled       bool
	autoNamespaces    valueobject.AllowedNamespaces
	version           string
	enabled           bool
}

func NewSource(namespace, name string, resourceType ResourceType) *Source {
	return &Source{
		id:                valueobject.NewSourceID(namespace, name),
		namespace:         namespace,
		name:              name,
		resourceType:      resourceType,
		allowed:           false,
		allowedNamespaces: valueobject.AllowedNamespaces{},
		autoEnabled:       false,
		autoNamespaces:    valueobject.AllowedNamespaces{},
		enabled:           true,
	}
}

func (s *Source) ID() valueobject.SourceID {
	return s.id
}

func (s *Source) Namespace() string {
	return s.namespace
}

func (s *Source) Name() string {
	return s.name
}

func (s *Source) TargetName() string {
	if s.targetName != "" {
		return s.targetName
	}
	return s.name
}

func (s *Source) SetTargetName(name string) {
	s.targetName = name
}

func (s *Source) ResourceType() ResourceType {
	return s.resourceType
}

func (s *Source) IsAllowed() bool {
	return s.allowed
}

func (s *Source) SetAllowed(allowed bool) {
	s.allowed = allowed
}

func (s *Source) AllowedNamespaces() valueobject.AllowedNamespaces {
	return s.allowedNamespaces
}

func (s *Source) SetAllowedNamespaces(namespaces valueobject.AllowedNamespaces) {
	s.allowedNamespaces = namespaces
}

func (s *Source) IsEnabled() bool {
	return s.enabled
}

func (s *Source) Disable() {
	s.enabled = false
}

func (s *Source) Enable() {
	s.enabled = true
}

func (s *Source) IsAutoEnabled() bool {
	return s.autoEnabled
}

func (s *Source) SetAutoEnabled(enabled bool) {
	s.autoEnabled = enabled
}

func (s *Source) AutoNamespaces() valueobject.AllowedNamespaces {
	return s.autoNamespaces
}

func (s *Source) SetAutoNamespaces(namespaces valueobject.AllowedNamespaces) {
	s.autoNamespaces = namespaces
}

func (s *Source) Version() string {
	return s.version
}

func (s *Source) SetVersion(v string) {
	s.version = v
}

func (s *Source) FullName() string {
	return fmt.Sprintf("%s/%s", s.namespace, s.name)
}

func (s *Source) CanReflectToNamespace(namespace string) bool {
	if !s.allowed {
		return false
	}
	return s.allowedNamespaces.Matches(namespace)
}

func (s *Source) CanAutoMirrorToNamespace(namespace string) bool {
	if !s.autoEnabled || !s.allowed {
		return false
	}
	if !s.autoNamespaces.IsEmpty() {
		return s.autoNamespaces.Matches(namespace)
	}
	return s.allowedNamespaces.Matches(namespace)
}

func (s *Source) ToObjectMeta() metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      s.name,
		Namespace: s.namespace,
	}
}

type Namespace struct {
	name    string
	deleted bool
}

func NewNamespace(name string) *Namespace {
	return &Namespace{
		name:    name,
		deleted: false,
	}
}

func (ns *Namespace) Name() string {
	return ns.name
}

func (ns *Namespace) IsDeleted() bool {
	return ns.deleted
}

func (ns *Namespace) MarkDeleted() {
	ns.deleted = true
}
