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
	"strings"

	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ClearHooks() error {
	ctx := context.Background()
	hookObjects, err := getHookObjects()
	if err != nil {
		return err
	}
	for _, hookObject := range hookObjects {
		err = k8sClient.Delete(ctx, hookObject)
		if err != nil {
			logger.Error(fmt.Sprintf("cannot delete hook object %s", hookObject.GetName()), zap.Error(err))
			return err
		}
		logger.Info(fmt.Sprintf("credential hook object %s has been deleted", hookObject.GetName()))
	}
	return nil
}

func getHookObjects() ([]client.Object, error) {
	resultList := make([]client.Object, 0)
	jobObjects, err := getJobsAndPods()
	if err != nil {
		return nil, err
	}
	credHookName := utils.GetHookName()
	for _, credHook := range jobObjects {
		if strings.HasPrefix(credHook.GetName(), credHookName) {
			resultList = append(resultList, credHook)
		}
	}

	return resultList, nil
}

func getJobsAndPods() ([]client.Object, error) {
	objects := make([]client.Object, 0)
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	jobList := &batchv1.JobList{}
	if err := k8sClient.List(context.Background(), jobList, opts...); err != nil {
		logger.Error("cannot get Job list", zap.Error(err))
		return nil, err
	}
	for _, job := range jobList.Items {
		objects = append(objects, &job)
	}

	podList := &corev1.PodList{}
	if err := k8sClient.List(context.Background(), podList, opts...); err != nil {
		logger.Error("cannot get Pod list", zap.Error(err))
		return nil, err
	}
	for _, pod := range podList.Items {
		objects = append(objects, &pod)
	}
	return objects, nil
}
