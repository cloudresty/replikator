package dto

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsHelmSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret *corev1.Secret
		want   bool
	}{
		{
			name: "helm.sh type secret",
			secret: &corev1.Secret{
				Type: "helm.sh/v1",
			},
			want: true,
		},
		{
			name: "regular secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Type: corev1.SecretTypeOpaque,
			},
			want: false,
		},
		{
			name: "secret with meta.helm.sh annotation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"meta.helm.sh/release-name": "my-release",
					},
				},
				Type: corev1.SecretTypeOpaque,
			},
			want: true,
		},
		{
			name: "secret with helm.sh/release annotation",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"helm.sh/release": "v1",
					},
				},
				Type: corev1.SecretTypeOpaque,
			},
			want: true,
		},
		{
			name: "secret with no annotations",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{},
				Type:       corev1.SecretTypeOpaque,
			},
			want: false,
		},
		{
			name: "secret with unrelated annotations",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"example.com/key": "value",
					},
				},
				Type: corev1.SecretTypeOpaque,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHelmSecret(tt.secret)
			if got != tt.want {
				t.Errorf("IsHelmSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHelmConfigMap(t *testing.T) {
	tests := []struct {
		name string
		cm   *corev1.ConfigMap
		want bool
	}{
		{
			name: "configmap with meta.helm.sh annotation",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"meta.helm.sh/release-name": "my-release",
					},
				},
			},
			want: true,
		},
		{
			name: "configmap with helm.sh/release annotation",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"helm.sh/release": "v1",
					},
				},
			},
			want: true,
		},
		{
			name: "regular configmap",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Data: map[string]string{
					"key": "value",
				},
			},
			want: false,
		},
		{
			name: "configmap with no annotations",
			cm: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{},
				Data: map[string]string{
					"key": "value",
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHelmConfigMap(tt.cm)
			if got != tt.want {
				t.Errorf("IsHelmConfigMap() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsHelmResource(t *testing.T) {
	tests := []struct {
		name string
		obj  interface{}
		want bool
	}{
		{
			name: "helm secret",
			obj: &corev1.Secret{
				Type: "helm.sh/v1",
			},
			want: true,
		},
		{
			name: "helm configmap",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"meta.helm.sh/release-name": "my-release",
					},
				},
			},
			want: true,
		},
		{
			name: "regular secret",
			obj: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Type: corev1.SecretTypeOpaque,
			},
			want: false,
		},
		{
			name: "regular configmap",
			obj: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Data: map[string]string{
					"key": "value",
				},
			},
			want: false,
		},
		{
			name: "unknown type",
			obj:  "unknown",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsHelmResource(tt.obj)
			if got != tt.want {
				t.Errorf("IsHelmResource() = %v, want %v", got, tt.want)
			}
		})
	}
}
