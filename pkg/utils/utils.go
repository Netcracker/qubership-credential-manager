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
	"encoding/json"
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

func GetLogger(level ...interface{}) *zap.Logger {
	logLevel := determineLogLevel(level...)
	atom := zap.NewAtomicLevel()
	encoderCfg := getEncoderConfig()
	zapLevel := getLogLevel(logLevel)

	customHandler := &CustomLogHandler{minLevel: zapLevel}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.AddSync(customHandler),
		atom,
	)

	baseFields := []zap.Field{
		zap.String("request_id", os.Getenv("REQUEST_ID")),
		zap.String("tenant_id", os.Getenv("TENANT_ID")),
		zap.String("thread", os.Getenv("THREAD")),
		zap.String("class", os.Getenv("CLASS")),
	}

	zapLogger := zap.New(core).With(baseFields...)
	atom.SetLevel(zapLevel)

	return zapLogger
}

func determineLogLevel(level ...interface{}) string {
	if len(level) > 0 {
		switch v := level[0].(type) {
		case string:
			return v
		case bool:
			if v {
				return "DEBUG"
			}
			return "INFO"
		}
	}
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "INFO"
	}
	return logLevel
}

func getLogLevel(level string) zapcore.Level {
	switch level {
	case "OFF":
		return zapcore.Level(6)
	case "FATAL":
		return zapcore.FatalLevel
	case "ERROR":
		return zapcore.ErrorLevel
	case "WARN":
		return zapcore.WarnLevel
	case "INFO":
		return zapcore.InfoLevel
	case "DEBUG":
		return zapcore.DebugLevel
	case "TRACE":
		return zapcore.Level(-1)
	default:
		return zapcore.InfoLevel
	}
}

func getEncoderConfig() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	return encoderCfg
}

type CustomLogHandler struct {
	minLevel zapcore.Level
}

func (h *CustomLogHandler) Write(p []byte) (n int, err error) {
	var logEntry map[string]interface{}
	if err := json.Unmarshal(p, &logEntry); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse log message: %s\n", p)
		return 0, fmt.Errorf("failed to parse log message")
	}

	levelStr := strings.ToUpper(fmt.Sprintf("%v", logEntry["level"]))
	timestamp := fmt.Sprintf("%v", logEntry["timestamp"])
	message := fmt.Sprintf("%v", logEntry["msg"])
	requestID := fmt.Sprintf("%v", logEntry["request_id"])
	tenantID := fmt.Sprintf("%v", logEntry["tenant_id"])
	thread := fmt.Sprintf("%v", logEntry["thread"])
	class := fmt.Sprintf("%v", logEntry["class"])

	output := fmt.Sprintf("[%s] [%s] [request_id=%s] [tenant_id=%s] [thread=%s] [class=%s] %s",
		timestamp, levelStr, requestID, tenantID, thread, class, message)

	fmt.Println(output)

	return len(p), nil
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
