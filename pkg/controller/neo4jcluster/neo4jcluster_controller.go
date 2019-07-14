// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package neo4jcluster

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	database "github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1"
	"github.com/lukasz-antoniak/neo4j-operator/pkg/backup"
	"github.com/lukasz-antoniak/neo4j-operator/pkg/reconciler"
	"io/ioutil"
	apps "k8s.io/api/apps/v1beta2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

var log = logf.Log.WithName("controller_neo4jcluster")

const ReconcileTime = 30 * time.Second

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Neo4jCluster Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	recon, err := newReconciler(mgr)
	if err != nil {
		return fmt.Errorf("failed to initialize Neo4J reconciler: %v", err)
	}
	return add(mgr, recon)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) (reconcile.Reconciler, error) {
	kubeClt, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}
	return &ReconcileNeo4jCluster{client: mgr.GetClient(), clientSet: kubeClt, scheme: mgr.GetScheme()}, nil
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("neo4jcluster-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Neo4jCluster
	err = c.Watch(&source.Kind{Type: &database.Neo4jCluster{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resources (StatefulSet, Service, Pod, Secrets)
	// and requeue the owner Neo4jCluster
	err = watchSecondaryResources(c, &apps.StatefulSet{}, &core.Service{}, &core.Pod{}, &core.Secret{})

	return err
}

func watchSecondaryResources(c controller.Controller, types ...runtime.Object) error {
	for _, t := range types {
		err := c.Watch(&source.Kind{Type: t}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &database.Neo4jCluster{},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var _ reconcile.Reconciler = &ReconcileNeo4jCluster{}

var managedObjects = []reconciler.ManagedObject {
	&reconciler.Secret{},
	&reconciler.CoreServer{},
	&reconciler.CoreService{},
	&reconciler.ReadReplica{},
	&reconciler.ReadReplicaService{},
}

// ReconcileNeo4jCluster reconciles a Neo4jCluster object
type ReconcileNeo4jCluster struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the api server
	client client.Client
	clientSet *kubernetes.Clientset
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Neo4jCluster object and makes changes based on the state read
// and what is in the Neo4jCluster.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNeo4jCluster) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	// reqLogger.Info("Reconciling Neo4jCluster started...")

	// Fetch the Neo4jCluster instance
	instance := &database.Neo4jCluster{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	err = backup.ScheduleBackup(r.clientSet, reqLogger, instance)
	if err != nil {
		reqLogger.Error(err, "Failed to schedule backup")
		return reconcile.Result{}, err
	}

	// TODO(lantonia): Build external service with ports (https, bolt) configurable. Can we really support internal and external access at once?
	for _, obj := range managedObjects {
		found := obj.DefaultObject()
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: obj.GetName(instance), Namespace: request.Namespace}, found)
		if err != nil && errors.IsNotFound(err) {
			result, err := obj.Create(instance)
			if err != nil {
				return reconcile.Result{}, err
			}
			if result != nil {
				reqLogger.Info(fmt.Sprintf("Creating new %T", result), "Namespace", result.GetNamespace(), "Name", result.GetName())
				err = r.client.Create(context.TODO(), result)
				if err != nil {
					reqLogger.Error(err, "Object creation failed", "Namespace", result.GetNamespace(), "Name", result.GetName())
					return reconcile.Result{}, err
				}
			}
		} else if err != nil {
			return reconcile.Result{}, err
		} else {
			result, restart, err := obj.Update(instance, found.DeepCopyObject())
			if err != nil {
				return reconcile.Result{}, err
			}
			if result == nil {
				reqLogger.Info(fmt.Sprintf("Deleting %T", result), "Namespace", result.GetNamespace(), "Name", result.GetName())
				err = r.client.Delete(context.TODO(), result)
				if err != nil {
					reqLogger.Error(err, "Object deletion failed", "Namespace", result.GetNamespace(), "Name", result.GetName())
					return reconcile.Result{}, err
				}
			} else if ! reflect.DeepEqual(result, found) {
				reqLogger.Info(fmt.Sprintf("Updating existing %T", result), "Namespace", result.GetNamespace(), "Name", result.GetName())
				err = r.client.Update(context.TODO(), result)
				if err != nil {
					reqLogger.Error(err, "Object update failed", "Namespace", result.GetNamespace(), "Name", result.GetName())
					return reconcile.Result{}, err
				}
				if restart {
					// We use rolling upgrade strategy, so there is no need to manually restart pods.
					//err = rollingPodRestart(reqLogger, r, instance)
					//if err != nil {
					//	reqLogger.Error(err, "Cluster rolling restart failed", "Namespace", result.GetNamespace(), "Name", result.GetName())
					//	return reconcile.Result{}, err
					//}
				}
			}
		}
	}

	err = r.updateClusterStatus(instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update cluster status")
		return reconcile.Result{}, err
	}

	// reqLogger.Info("Reconciliation of Neo4jCluster completed.")
	return reconcile.Result{RequeueAfter: ReconcileTime}, nil
}

func rollingPodRestart(logger logr.Logger, r *ReconcileNeo4jCluster, instance *database.Neo4jCluster) error {
	foundPods := &core.PodList{}
	listOps := &client.ListOptions{
		Namespace:     instance.Namespace,
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"component": instance.LabelComponentName(),
		}),
	}
	err := r.client.List(context.TODO(), listOps, foundPods)
	if err != nil {
		return err
	}
	for _, p := range foundPods.Items {
		err = restartPod(logger, r, instance, p)
		if err != nil {
			return err
		}
	}
	return nil
}

func restartPod(logger logr.Logger, r *ReconcileNeo4jCluster, instance *database.Neo4jCluster, pod core.Pod) error {
	logger.Info(fmt.Sprintf("Restarting pod %s.", pod.Name))
	err := r.clientSet.CoreV1().Pods(instance.Namespace).Delete(pod.Name, &meta.DeleteOptions{})
	if err != nil {
		return err
	}
	for {
		refreshed, err := r.clientSet.CoreV1().Pods(instance.Namespace).Get(pod.Name, meta.GetOptions{IncludeUninitialized: true})
		if err != nil {
			return err
		}
		if refreshed.DeletionTimestamp == nil {
			ready := true
			for _, c := range refreshed.Status.ContainerStatuses {
				if !c.Ready {
					ready = false
				}
			}
			if ready {
				return nil
			}
		} else {
			// Pod is being removed, wait.
		}
		logger.Info(fmt.Sprintf("Waiting for pod %s to become ready...", refreshed.Name))
		time.Sleep(10 * time.Second)
	}
}

func (r *ReconcileNeo4jCluster) updateClusterStatus(instance *database.Neo4jCluster) error {
	coreSelector := labels.SelectorFromSet(map[string]string{
		"component": instance.LabelComponentName(),
		"role": "neo4j-core",
	})
	replicaSelector := labels.SelectorFromSet(map[string]string{
		"component": instance.LabelComponentName(),
		"role": "neo4j-replica",
	})
	cOn, cOff, err := findOnlineOfflinePods(r, instance.Namespace, coreSelector)
	if err != nil {
		return err
	}
	rOn, rOff, err := findOnlineOfflinePods(r, instance.Namespace, replicaSelector)
	if err != nil {
		return err
	}
	if ! instance.Spec.IsCausalCluster() {
		instance.Status.Leader = "N/A"
	} else {
		instance.Status.Leader = discoverLeader(instance, cOn)
	}
	instance.Status.CoreStats = fmt.Sprintf("%d/%d", len(cOn), len(cOn) + len(cOff))
	instance.Status.ReplicaStats = fmt.Sprintf("%d/%d", len(rOn), len(rOn) + len(rOff))
	if ! instance.Spec.IsCausalCluster() {
		instance.Status.BoltURL = fmt.Sprintf("bolt://neo4j-core-%s:7687", instance.Name)
	} else {
		instance.Status.BoltURL = fmt.Sprintf("bolt+routing://neo4j-core-%s:7687", instance.Name)
	}
	instance.Status.State = ""
	instance.Status.Message = ""
	return r.client.Status().Update(context.TODO(), instance)
}

// TODO(lantonia): Refactor methods so that you do not pass Neo4JCluster object everywhere.
func discoverLeader(instance *database.Neo4jCluster, corePods []string) string {
	leader := ""
	adminPassword, _ := instance.Spec.AdminPasswordClearText()
	ch := make(chan string)
	for _, coreServer := range corePods {
		go func(ch chan string, instance string, password *string) {
			httpClient := http.Client{Timeout: 5 * time.Second}
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://%s:7474/db/manage/server/core/writable", instance), nil)
			if password != nil {
				req.SetBasicAuth("neo4j", *password)
			}
			res, err := httpClient.Do(req);
			if err == nil {
				defer res.Body.Close()
			}
			if err != nil || res.StatusCode != http.StatusOK {
				ch <- ""
			} else {
				body, _ := ioutil.ReadAll(res.Body)
				if string(body) != "true" {
					ch <- ""
				} else {
					ch <- instance
				}
			}
		}(ch, fmt.Sprintf("%s.%s", coreServer, instance.CoreServiceName()), adminPassword)
	}
	for range corePods {
		if response := <-ch; response != "" {
			leader = response
		}
	}
	return leader
}

func findOnlineOfflinePods(r *ReconcileNeo4jCluster, namespace string, labelSelector labels.Selector) ([]string, []string, error) {
	foundPods := &core.PodList{}
	listOps := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), listOps, foundPods)
	if err != nil {
		return nil, nil, err
	}
	var (
		readyMembers []string
		unreadyMembers []string
	)
	for _, p := range foundPods.Items {
		ready := true
		for _, c := range p.Status.ContainerStatuses {
			if !c.Ready {
				ready = false
			}
		}
		if ready {
			readyMembers = append(readyMembers, p.Name)
		} else {
			unreadyMembers = append(unreadyMembers, p.Name)
		}
	}
	return readyMembers, unreadyMembers, nil
}
