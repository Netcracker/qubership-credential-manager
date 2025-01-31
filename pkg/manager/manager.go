// Copyright 2024-2025 NetCracker Technology Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"sync"

	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	lockLabel = "locked-for-watcher"
)

var (
	namespace = utils.GetNamespace()
	logger    = utils.GetLogger()

	k8sClientInstance client.Client
	once              sync.Once
)

func GetK8SClient() client.Client {
	once.Do(func() {
		k8sClientInstance = utils.GetK8SClient()
	})
	return k8sClientInstance
}

func AreCredsChanged(secretNames []string) (bool, error) {
	for _, secretName := range secretNames {
		newSecret, err := getSecret(secretName)
		if err != nil {
			return false, err
		}
		oldSecretName := utils.GetOldSecretName(secretName)
		oldSecret, err := getSecret(oldSecretName)
		if err != nil {
			return false, err
		}
		if utils.AreFieldsChanged(oldSecret, newSecret) {
			return true, nil
		}
	}
	return false, nil
}

func ActualizeCreds(secretName string, changeCredsFunc func(newSecret, oldSecret *corev1.Secret) error) (err error) {
	defer func() {
		if err == nil {
			err = unlockSecret(secretName)
			if err != nil {
				logger.Error("Credentials secret wasn't unlocked", zap.Error(err))
			}
		}
	}()

	newSecret, err := getSecret(secretName)
	if err != nil {
		return
	}
	oldSecretName := utils.GetOldSecretName(secretName)
	oldSecret, err := getSecret(oldSecretName)
	if err != nil {
		if errors.IsNotFound(err) {
			oldSecret := getNewSecret(oldSecretName)
			oldSecret.Data = newSecret.Data
			err = createSecret(oldSecret)
			return
		}
		return
	}

	if !utils.AreFieldsChanged(oldSecret, newSecret) {
		return
	}

	err = changeCredsFunc(newSecret, oldSecret)
	if err != nil {
		return
	}

	oldSecret.Data = newSecret.Data
	err = updateSecret(oldSecret)
	return
}

func getNewSecret(secretName string) *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
	}
}

func unlockSecret(secretName string) error {
	logger.Info("Secret will be unlocked")
	secret, err := getSecret(secretName)
	if err != nil {
		return err
	}
	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Annotations[lockLabel] = "false"
	return GetK8SClient().Update(context.Background(), secret)
}

func createSecret(secret *corev1.Secret) error {
	err := GetK8SClient().Create(context.TODO(), secret)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create secret %v", secret.ObjectMeta.Name), zap.Error(err))
		return err
	}
	return nil
}

func updateSecret(secret *corev1.Secret) error {
	err := GetK8SClient().Update(context.TODO(), secret)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to update secret %v", secret.ObjectMeta.Name), zap.Error(err))
		return err
	}
	return nil
}

func SetOwnerRefForSecretCopies(secretNames []string, ownerRef []metav1.OwnerReference) error {
	for _, secretName := range secretNames {
		oldSecretName := utils.GetOldSecretName(secretName)
		secret, err := getSecret(oldSecretName)
		if err != nil {
			return err
		}
		secret.OwnerReferences = ownerRef
		err = updateSecret(secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddCredHashToPodTemplate(secretNames []string, template *corev1.PodTemplateSpec) error {
	for i, secretName := range secretNames {
		patroniHash, err := CalculateSecretDataHash(secretName)
		if err != nil {
			return err
		}
		annotations := map[string]string{
			GetAnnotationName(i): patroniHash,
		}
		AddAnnotationsToPodTemplate(template, annotations)
	}
	return nil
}

func GetAnnotationName(id int) string {
	return fmt.Sprintf("checksum/secret%d", id)
}

func AddAnnotationsToPodTemplate(template *corev1.PodTemplateSpec, annotations map[string]string) {
	if template.Annotations == nil {
		template.Annotations = annotations
	} else {
		for key, value := range annotations {
			template.Annotations[key] = value
		}
	}
}

func CalculateSecretDataHash(secretName string) (string, error) {
	secret, err := getSecret(secretName)
	if err != nil {
		return "", err
	}
	return hash(secret.Data)
}

func getSecret(secretName string) (*corev1.Secret, error) {
	foundSecret := &corev1.Secret{}
	err := GetK8SClient().Get(context.TODO(), types.NamespacedName{
		Name: secretName, Namespace: namespace,
	}, foundSecret)
	if err != nil {
		logger.Error(fmt.Sprintf("can't find the secret %s", secretName), zap.Error(err))
		return foundSecret, err
	}
	return foundSecret, nil
}

// Hash returns hash SHA-256 of object
func hash(o interface{}) (string, error) {
	cr, err := json.Marshal(o)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	hash.Write(cr)
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
