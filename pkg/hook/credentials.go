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

package hook

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	logger    = utils.GetLogger()
	k8sClient = utils.GetK8SClient()
	namespace = utils.GetNamespace()
)

func PrepareOldCreds(secrets []string) {
	for _, secretName := range secrets {
		oldSecretName := fmt.Sprintf("%s-old", secretName)
		logger.Info(fmt.Sprintf("Creation of secret %s was started", oldSecretName))
		ctx := context.Background()

		newSecret := &corev1.Secret{}
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name: secretName, Namespace: namespace,
		}, newSecret)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.Info(fmt.Sprintf("secret %s is not found, skipping...", secretName))
				continue
			}
			logger.Info(fmt.Sprintf("cannot get %s secret", secretName))
			panic(err)
		}
		if isSecretLocked(newSecret) {
			logger.Info("Secret is locked, skip old secret update...")
			continue
		}

		isSecretExist, err := IsSecretExist(oldSecretName)
		if err != nil {
			panic(err)
		}
		oldSecret := oldSecret(oldSecretName)
		oldSecret.Data = newSecret.Data
		oldSecret.Labels = newSecret.Labels
		if !isSecretExist {
			err = k8sClient.Create(ctx, oldSecret)
			if err != nil {
				logger.Info(fmt.Sprintf("cannot create %s secret", oldSecret.Name))
				panic(err)
			}
		} else {
			err = k8sClient.Update(ctx, oldSecret)
			if err != nil {
				logger.Info(fmt.Sprintf("cannot update %s secret", oldSecret.Name))
				panic(err)
			}
		}

		annotations := map[string]string{
			utils.LockLabel: "true",
		}
		if newSecret.Annotations == nil {
			newSecret.Annotations = annotations
		} else {
			for key, value := range annotations {
				newSecret.Annotations[key] = value
			}
		}
		err = k8sClient.Update(ctx, newSecret)
		if err != nil {
			logger.Info(fmt.Sprintf("cannot update %s secret", newSecret.Name))
			panic(err)
		}
	}
}

func isSecretLocked(secret *corev1.Secret) bool {
	return secret.Annotations[utils.LockLabel] == "true"
}

func IsSecretExist(name string) (bool, error) {
	newSecret := &corev1.Secret{}
	err := k8sClient.Get(context.Background(), types.NamespacedName{
		Name: name, Namespace: namespace,
	}, newSecret)
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		logger.Info(fmt.Sprintf("cannot get %s secret", name))
		return false, err
	}
	return true, nil
}

func oldSecret(oldSecretName string) *corev1.Secret {
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Name:      oldSecretName,
			Namespace: namespace,
			Labels:    commonLabels(oldSecretName),
		},
	}
}

func IsHook() bool {
	isHookStr := utils.GetEnv("IS_HOOK", "false")
	isHook, err := strconv.ParseBool(isHookStr)
	if err != nil {
		panic(err)
	}
	return isHook
}

func commonLabels(name string) map[string]string {
	sessionId := strings.ToLower(os.Getenv("SESSION_ID"))
	applicationName := strings.ToLower(os.Getenv("APPLICATION_NAME"))

	labels := map[string]string{
		"app.kubernetes.io/managed-by": "credential-manager",
	}
	if sessionId != "" {
		labels["deployment.netcracker.com/sessionId"] = sessionId
	}
	if applicationName != "" {
		labels["app.kubernetes.io/part-of"] = applicationName
	}

	return labels
}
