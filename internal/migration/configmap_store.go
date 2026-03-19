package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	cmPrefix = "aap-migration-"
	cmLabel  = "app.kubernetes.io/component"
	cmValue  = "migration-state"
)

// ConfigMapStore persists migration state in K8s ConfigMaps.
// Each migration gets its own ConfigMap: aap-migration-{id}.
// This survives pod restarts unlike emptyDir.
type ConfigMapStore struct {
	client    kubernetes.Interface
	namespace string
}

// NewConfigMapStore creates a K8s ConfigMap-backed migration store.
// Returns nil if not running in K8s.
func NewConfigMapStore() *ConfigMapStore {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		slog.Error("failed to create k8s client for migration state", "error", err)
		return nil
	}

	namespace := os.Getenv("AAP_NAMESPACE")
	if namespace == "" {
		namespace = "argocd-addons-platform"
	}

	slog.Info("migration state will be stored in K8s ConfigMaps", "namespace", namespace)
	return &ConfigMapStore{
		client:    clientset,
		namespace: namespace,
	}
}

func (cs *ConfigMapStore) cmName(id string) string {
	return cmPrefix + id
}

// SaveMigration stores migration state in a ConfigMap.
func (cs *ConfigMapStore) SaveMigration(ctx context.Context, m *Migration) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshaling migration: %w", err)
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cs.cmName(m.ID),
			Namespace: cs.namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "aap",
				cmLabel:                        cmValue,
			},
		},
		Data: map[string]string{
			"migration.json": string(data),
		},
	}

	_, err = cs.client.CoreV1().ConfigMaps(cs.namespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		_, err = cs.client.CoreV1().ConfigMaps(cs.namespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("saving migration %s: %w", m.ID, err)
		}
	}
	return nil
}

// GetMigration reads migration state from a ConfigMap.
func (cs *ConfigMapStore) GetMigration(ctx context.Context, id string) (*Migration, error) {
	cm, err := cs.client.CoreV1().ConfigMaps(cs.namespace).Get(ctx, cs.cmName(id), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("reading migration %s: %w", id, err)
	}

	data, ok := cm.Data["migration.json"]
	if !ok {
		return nil, fmt.Errorf("migration %s has no data", id)
	}

	var m Migration
	if err := json.Unmarshal([]byte(data), &m); err != nil {
		return nil, fmt.Errorf("decoding migration %s: %w", id, err)
	}
	return &m, nil
}

// ListMigrations returns all migrations stored as ConfigMaps.
func (cs *ConfigMapStore) ListMigrations(ctx context.Context) ([]*Migration, error) {
	cmList, err := cs.client.CoreV1().ConfigMaps(cs.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: cmLabel + "=" + cmValue,
	})
	if err != nil {
		return nil, fmt.Errorf("listing migration configmaps: %w", err)
	}

	var migrations []*Migration
	for _, cm := range cmList.Items {
		data, ok := cm.Data["migration.json"]
		if !ok {
			continue
		}
		var m Migration
		if err := json.Unmarshal([]byte(data), &m); err != nil {
			continue
		}
		migrations = append(migrations, &m)
	}
	return migrations, nil
}

// DeleteMigration removes a migration ConfigMap.
func (cs *ConfigMapStore) DeleteMigration(ctx context.Context, id string) error {
	return cs.client.CoreV1().ConfigMaps(cs.namespace).Delete(ctx, cs.cmName(id), metav1.DeleteOptions{})
}

// HasActiveMigration checks if any migration is in an active state.
func (cs *ConfigMapStore) HasActiveMigration(ctx context.Context) (bool, string, error) {
	migrations, err := cs.ListMigrations(ctx)
	if err != nil {
		return false, "", err
	}
	for _, m := range migrations {
		if m.Status == StatusRunning || m.Status == StatusWaiting || m.Status == StatusPaused || m.Status == StatusGated {
			return true, m.ID, nil
		}
	}
	return false, "", nil
}

// isK8sAvailable checks if this store was initialized.
func isK8sConfigMapAvailable(cs *ConfigMapStore) bool {
	return cs != nil
}

// migrateFromPrefix extracts migration ID from ConfigMap name.
func migrateIDFromCMName(name string) string {
	return strings.TrimPrefix(name, cmPrefix)
}
