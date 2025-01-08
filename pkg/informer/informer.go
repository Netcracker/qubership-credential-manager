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

package informer

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/Netcracker/qubership-credential-manager/pkg/utils"
	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logger    = utils.GetLogger()
	namespace = utils.GetNamespace()

	activeWatchers = make(map[string]*Watcher)
	mutex          = sync.Mutex{}
	once           sync.Once
	k8sClient      client.Client
)

func GetK8SClient() client.Client {
	once.Do(func() {
		k8sClient = utils.GetK8SClient()
	})
	return k8sClient
}

type Watcher struct {
	secretName    string
	informer      cache.SharedInformer
	reconcileFunc func()
}

func (w Watcher) Start() {
	// Prepare watcher clean
	stopCh := make(chan struct{})
	defer func() {
		mutex.Lock()
		close(stopCh)
		delete(activeWatchers, w.secretName)
		mutex.Unlock()
	}()

	//Start active watcher
	logger.Info("Creds watcher started")
	w.informer.Run(stopCh)
	logger.Info("Creds watcher finished")
}

func newWatcher(secretName string, reconcileFunc func()) (*Watcher, error) {
	namespace := namespace
	clientSet := getKubeClient()
	if reconcileFunc == nil {
		return nil, fmt.Errorf("no reconcile function was provided")
	}
	secretFields := map[string]string{"metadata.name": secretName}
	informer := cache.NewSharedInformer(
		&cache.ListWatch{
			ListFunc: func(opts metav1.ListOptions) (runtime.Object, error) {
				secretsList := &corev1.SecretList{}
				listOps := &client.ListOptions{
					FieldSelector: fields.SelectorFromSet(secretFields),
					Namespace:     namespace,
				}
				err := GetK8SClient().List(context.Background(), secretsList, listOps)
				return secretsList, err
			},
			WatchFunc: func(opts metav1.ListOptions) (watch.Interface, error) {
				return clientSet.CoreV1().Secrets(namespace).Watch(context.Background(), metav1.ListOptions{
					FieldSelector: fields.SelectorFromSet(secretFields).String(),
				})
			},
		},
		&corev1.Secret{},
		1*time.Hour, //TODO: check
	)

	w := &Watcher{secretName: secretName, informer: informer, reconcileFunc: reconcileFunc}

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: w.credsUpdFunc,
	})
	if err != nil {
		logger.Error("Cannot register credentials handler function", zap.Error(err))
		return nil, err
	}

	return w, nil
}

func (w *Watcher) credsUpdFunc(oldObj, newObj interface{}) {
	oldSecret, ok := oldObj.(*corev1.Secret)
	if !ok {
		errMsg := "old watched credentials secret is not Secret object"
		logger.Error(errMsg)
		return
	}
	newSecret, ok := newObj.(*corev1.Secret)
	if !ok {
		errMsg := "new watched credentials secret is not Secret object"
		logger.Error(errMsg)
		return
	}
	if locked := newSecret.Annotations[utils.LockLabel]; locked == "true" {
		logger.Info("Creds secret is locked by update job, skip password change procedure")
		return
	} else if locked := oldSecret.Annotations[utils.LockLabel]; locked == "true" {
		logger.Info("Creds secret just was unlocked, skip password change procedure")
		return
	}

	if utils.AreFieldsChanged(oldSecret, newSecret) {
		logger.Info("New credentials found, starting reconcile...")
		w.reconcileFunc()
	}
}

func Watch(secretNames []string, reconcileFunc func()) error {
	mutex.Lock()
	defer mutex.Unlock()
	for _, secretName := range secretNames {
		// Init watcher
		watcher := activeWatchers[secretName]

		if watcher == nil {
			var err error
			watcher, err = newWatcher(secretName, reconcileFunc)
			if err != nil {
				return err
			}
			activeWatchers[secretName] = watcher
		} else {
			logger.Info(fmt.Sprintf("Active watcher for secret %s already exist", secretName))
			continue
		}
		go watcher.Start()
	}

	return nil
}

func getKubeClient() *kubernetes.Clientset {
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubec", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubec", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		// use the current context in kubeconfig
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err.Error())
		}

	}
	k8sConfig.Timeout = 60 * time.Second
	client, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		panic(err)
	}
	return client
}
