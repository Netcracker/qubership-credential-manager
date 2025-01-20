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

package utils

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const LockLabel = "locked-for-watcher"

const nsPath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var (
	logger    *zap.Logger
	k8sClient client.Client
)

func GetLogger() *zap.Logger {
	if logger == nil {
		logger = createLogger()
	}

	return logger
}

func createLogger() *zap.Logger {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	newLogger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	defer func() {
		_ = newLogger.Sync()
	}()
	return newLogger
}

func GetK8SClient() client.Client {
	if k8sClient == nil {
		k8sClient = createClient()
	}

	return k8sClient
}

func createClient() client.Client {
	clientConfig, err := config.GetConfig()
	if err != nil {
		panic(err.Error())
	}
	client, err := client.New(clientConfig, client.Options{})
	if err != nil {
		panic(err.Error())
	}
	return client
}

func GetNamespace() string {
	namespace, err := ReadFromFile(nsPath)
	if err != nil {
		//try read namespace from env var
		namespace = os.Getenv("NAMESPACE")
		if namespace == "" {
			logger.Error("namespace can't be extracted", zap.Error(err))
			panic(err)
		}
	}
	return namespace
}

func ReadFromFile(filePath string) (string, error) {
	dat, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

func GetEnv(key string, def string) string {
	v := os.Getenv(key)
	if len(v) == 0 {
		return def
	}
	return v
}

func AreFieldsChanged(oldSecret, newSecret *corev1.Secret) bool {
	isChanged := false
	for fieldName := range newSecret.Data {
		if string(oldSecret.Data[fieldName]) != string(newSecret.Data[fieldName]) {
			isChanged = true
		}
	}
	return isChanged
}

func GetOldSecretName(secretName string) string {
	return fmt.Sprintf("%s-old", secretName)
}

func GetSecretNames() []string {
	secretNamesStr := os.Getenv("SECRET_NAMES")
	secretNames := strings.Split(secretNamesStr, ",")
	return secretNames
}

func GetHookName() string {
	return GetEnv("HOOK_NAME", "credentials-saver")
}
