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
	"encoding/base64"
	neo4jv1alpha1 "github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1"
	appsv1 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"testing"
)

func TestNeo4JClusterController(t *testing.T) {
	// Set the logger to development mode for verbose logs.
	logf.SetLogger(logf.ZapLogger(true))

	var (
		name = "example1"
		namespace = "db"
		version = "3.5.4-enterprise"
		core int32 = 3
		replicas int32 = 3
		password = "Neo4J#Passw0rd123!"
	)

	// A Neo4J resource with metadata and spec.
	neo4j := &neo4jv1alpha1.Neo4jCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: neo4jv1alpha1.Neo4jClusterSpec{
			CoreServers: core,
			ReadReplicaServers: replicas,
			ImageVersion: version,
			AdminPassword: base64.StdEncoding.EncodeToString( []byte(password) ),
		},
	}
	// Objects to track in the fake client.
	objs := []runtime.Object{ neo4j }

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(neo4jv1alpha1.SchemeGroupVersion, neo4j)
	// Create a fake client to mock API calls.
	cl := fake.NewFakeClient(objs...)
	// Create a ReconcileNeo4jCluster object with the scheme and fake client.
	r := &ReconcileNeo4jCluster{client: cl, scheme: s}

	// Mock request to simulate Reconcile() being called on an event for a
	// watched resource .
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      name,
			Namespace: namespace,
		},
	}
	res, err := r.Reconcile(req)
	if err != nil {
		t.Fatalf("reconcile: (%v)", err)
	}
	// Check the result of reconciliation to make sure it has the desired state.
	if res.RequeueAfter == 0 {
		t.Error("reconcile did not requeue request as expected")
	}

	// Check if core stateful set has been created and has the correct size.
	ss := &appsv1.StatefulSet{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: neo4j.CoreServiceName(), Namespace: req.Namespace}, ss)
	if err != nil {
		t.Fatalf("get core stateful set: (%v)", err)
	}
	size := *ss.Spec.Replicas
	if size != core {
		t.Errorf("core stateful set size (%d) is not as expected (%d)", size, core)
	}

	// Check if core service has been created.
	srv := &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: neo4j.CoreServiceName(), Namespace: req.Namespace}, srv)
	if err != nil {
		t.Fatalf("get core service: (%v)", err)
	}

	// Check if replica stateful set has been created and has the correct size.
	err = cl.Get(context.TODO(), types.NamespacedName{Name: neo4j.ReadReplicaName(), Namespace: req.Namespace}, ss)
	if err != nil {
		t.Fatalf("get replica stateful set: (%v)", err)
	}
	size = *ss.Spec.Replicas
	if size != core {
		t.Errorf("replica stateful set size (%d) is not as expected (%d)", size, replicas)
	}

	// Check if replica service has been created.
	srv = &corev1.Service{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: neo4j.ReadReplicaName(), Namespace: req.Namespace}, srv)
	if err != nil {
		t.Fatalf("get replica service: (%v)", err)
	}

	// Check if administrator password has been created properly.
	pwd := &corev1.Secret{}
	err = cl.Get(context.TODO(), types.NamespacedName{Name: neo4j.SecretStoreName(), Namespace: req.Namespace}, pwd)
	if err != nil {
		t.Fatalf("get secret store: (%v)", err)
	}
	if string(pwd.Data["neo4j-password"]) != password {
		t.Fatalf("invalid password: (%v)", string(pwd.Data["neo4j-password"]))
	}
}