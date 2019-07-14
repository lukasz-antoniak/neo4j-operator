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

package reconciler

import (
	"encoding/base64"
	"fmt"
	database "github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1"
	apps "k8s.io/api/apps/v1beta2"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"reflect"
	"strconv"
)

type CoreServer struct {
}

func (s *CoreServer) Create(instance *database.Neo4jCluster) (MetaObject, error) {
	return buildCoreServers(instance)
}

func (s *CoreServer) Update(instance *database.Neo4jCluster, found runtime.Object) (MetaObject, bool, error) {
	other := found.(*apps.StatefulSet)
	tmp, _ := buildCoreServers(instance)
	restart := false

	// Override Docker image version
	other.Spec.Template.Spec.Containers[0].Image = tmp.Spec.Template.Spec.Containers[0].Image

	// Override Docker image pull policy
	other.Spec.Template.Spec.Containers[0].ImagePullPolicy = tmp.Spec.Template.Spec.Containers[0].ImagePullPolicy

	// Scale up or down the cluster
	other.Spec.Replicas = &instance.Spec.CoreServers

	// Updating environment variables
	if ! reflect.DeepEqual(tmp.Spec.Template.Spec.Containers[0].Env, other.Spec.Template.Spec.Containers[0].Env) {
		other.Spec.Template.Spec.Containers[0].Env = tmp.Spec.Template.Spec.Containers[0].Env
		restart = true
	}

	// Update CPU and memory resources
	if ! equalResources(&tmp.Spec.Template.Spec.Containers[0].Resources, &other.Spec.Template.Spec.Containers[0].Resources) {
		other.Spec.Template.Spec.Containers[0].Resources = tmp.Spec.Template.Spec.Containers[0].Resources
	}

	return other, restart, nil
}

func equalResources(x *core.ResourceRequirements, y *core.ResourceRequirements) bool {
	return x.Requests.Cpu().Cmp(*y.Requests.Cpu()) == 0 && x.Requests.Memory().Cmp(*y.Requests.Memory()) == 0 &&
		x.Limits.Cpu().Cmp(*y.Limits.Cpu()) == 0 && x.Limits.Memory().Cmp(*y.Limits.Memory()) == 0
}

func (s *CoreServer) GetName(instance *database.Neo4jCluster) string {
	return instance.CoreServiceName()
}

func (s *CoreServer) DefaultObject() runtime.Object {
	return &apps.StatefulSet{}
}

func buildCoreServers(instance *database.Neo4jCluster) (*apps.StatefulSet, error) {
	// TODO(lantonia): Add more unit tests.
	imagePullPolicy := instance.Spec.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = "IfNotPresent"
	}
	limitCpu, _ := resource.ParseQuantity(instance.Spec.Resources.Limits.CPU)
	limitMemory, _ := resource.ParseQuantity(instance.Spec.Resources.Limits.Memory)
	requestCpu, _ := resource.ParseQuantity(instance.Spec.Resources.Requests.CPU)
	requestMemory, _ := resource.ParseQuantity(instance.Spec.Resources.Requests.Memory)
	dataMountPath := "/data"
	if instance.Spec.PersistentStorage != nil {
		if instance.Spec.PersistentStorage.MountPath != "" {
			dataMountPath = instance.Spec.PersistentStorage.MountPath
		}
	}
	defaultLabels := map[string]string{
		"component": instance.LabelComponentName(),
		"role":      "neo4j-core",
	}
	probe := &core.Probe{
		Handler: core.Handler{
			HTTPGet: &core.HTTPGetAction{
				Scheme: core.URISchemeHTTP,
				Path: "/db/manage/server/core/available",
				Port: intstr.FromInt(7474),
			},
		},
		InitialDelaySeconds: 180,
		TimeoutSeconds: 2,
		PeriodSeconds: 10,
		SuccessThreshold: 1,
		FailureThreshold: 3,
	}
	if instance.Spec.AuthorizationEnabled() {
		password, _ := instance.Spec.AdminPasswordClearText()
		probe.HTTPGet.HTTPHeaders = []core.HTTPHeader{
			{
				Name: "Authorization",
				Value: fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("neo4j:%s", *password)))),
			},
		}
	}
	statefulSet := &apps.StatefulSet{
		ObjectMeta: meta.ObjectMeta{
			Name:      instance.CoreServiceName(),
			Namespace: instance.Namespace,
			Labels:    defaultLabels,
		},
		Spec: apps.StatefulSetSpec{
			Replicas:    &instance.Spec.CoreServers,
			PodManagementPolicy: "Parallel",
			UpdateStrategy: apps.StatefulSetUpdateStrategy{
				Type: apps.RollingUpdateStatefulSetStrategyType,
			},
			ServiceName: instance.CoreServiceName(),
			Selector: &meta.LabelSelector{
				MatchLabels: defaultLabels,
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: meta.ObjectMeta{
					Labels: defaultLabels,
				},
				Spec: core.PodSpec{
					NodeSelector: instance.Spec.NodeSelector,
					Containers: []core.Container{
						{
							Name:            "core",
							Image:           instance.Spec.DockerImage(),
							ImagePullPolicy: core.PullPolicy(imagePullPolicy),
							Env: []core.EnvVar{
								{Name: "NEO4J_ACCEPT_LICENSE_AGREEMENT", Value: "yes"},
								{Name: "AUTH_ENABLED", Value: strconv.FormatBool(instance.Spec.AuthorizationEnabled())},
								{Name: "SSL_CERTIFICATES", Value: strconv.FormatBool(instance.Spec.SslCertificates != nil)},
								{Name: "NUMBER_OF_CORES", Value: strconv.FormatInt(int64(instance.Spec.CoreServers), 10)},
							},
							ReadinessProbe: probe,
							LivenessProbe: probe,
							Command: []string{
								"/bin/bash",
								"-c",
								`
export NEO4J_dbms_connectors_default__advertised__address=$(hostname -f)
export NEO4J_causal__clustering_discovery__advertised__address=$(hostname -f):5000
export NEO4J_causal__clustering_transaction__advertised__address=$(hostname -f):6000
export NEO4J_causal__clustering_raft__advertised__address=$(hostname -f):7000
export NEO4J_dbms_connector_bolt_listen__address=0.0.0.0:7687
export NEO4J_dbms_connector_http_listen__address=0.0.0.0:7474
export NEO4J_dbms_connector_https_enabled=true            
export NEO4J_dbms_connector_https_listen__address=0.0.0.0:7473
export NEO4J_dbms_connector_bolt_tls__level=OPTIONAL
export NEO4J_dbms_backup_enabled=true
export NEO4J_dbms_backup_address=0.0.0.0:6362
if [ "${AUTH_ENABLED:-}" == "true" ]; then
  export NEO4J_AUTH="neo4j/${NEO4J_SECRETS_PASSWORD}"
else
  export NEO4J_AUTH="none"
fi
if [ "${SSL_CERTIFICATES:-}" == "true" ]; then
  mkdir /ssl
  echo "${SSL_KEY}" > /ssl/neo4j.key
  echo "${SSL_CERTIFICATE}" > /ssl/neo4j.cert
fi
rm -rf /var/lib/neo4j/data/dbms/auth
exec /docker-entrypoint.sh "neo4j"
`,
							},
							Ports: []core.ContainerPort{
								{Name: "discovery", ContainerPort: 5000, Protocol: "TCP"},
								{Name: "tx", ContainerPort: 6000, Protocol: "TCP"},
								{Name: "raft", ContainerPort: 7000, Protocol: "TCP"},
								{Name: "browser-https", ContainerPort: 7473, Protocol: "TCP"},
								{Name: "browser-http", ContainerPort: 7474, Protocol: "TCP"},
								{Name: "bolt", ContainerPort: 7687, Protocol: "TCP"},
							},
							VolumeMounts: []core.VolumeMount{
								{Name: "datadir", MountPath: dataMountPath},
								{Name: "plugins", MountPath: "/plugins"},
							},
							Resources: core.ResourceRequirements{
								Limits: core.ResourceList{
									"cpu":    limitCpu,
									"memory": limitMemory,
								},
								Requests: core.ResourceList{
									"cpu":    requestCpu,
									"memory": requestMemory,
								},
							},
						},
					},
					Volumes: []core.Volume{
						{
							Name:         "plugins",
							VolumeSource: core.VolumeSource{EmptyDir: &core.EmptyDirVolumeSource{},},
						},
					},
				},
			},
		},
	}
	templateSpec := &statefulSet.Spec.Template.Spec
	if instance.Spec.AuthorizationEnabled() {
		secret := core.EnvVar{
			Name: "NEO4J_SECRETS_PASSWORD",
			ValueFrom: &core.EnvVarSource{
				SecretKeyRef: &core.SecretKeySelector{
					LocalObjectReference: core.LocalObjectReference{ Name: instance.SecretStoreName() },
					Key: "neo4j-password",
				},
			},
		}
		templateSpec.Containers[0].Env = append(templateSpec.Containers[0].Env, secret)
	}
	if instance.Spec.SslCertificates != nil {
		key := core.EnvVar{
			Name: "SSL_KEY",
			Value: instance.Spec.SslCertificates.PrivateKey,
		}
		certificate := core.EnvVar{
			Name: "SSL_CERTIFICATE",
			Value: instance.Spec.SslCertificates.PublicCertificate,
		}
		templateSpec.Containers[0].Env = append(templateSpec.Containers[0].Env, key, certificate)
	}
	if ! instance.Spec.IsCausalCluster() {
		templateSpec.Containers[0].Env = append(templateSpec.Containers[0].Env, core.EnvVar{Name: "NEO4J_dbms_mode", Value: "SINGLE"})
	} else {
		templateSpec.Containers[0].Env = append(templateSpec.Containers[0].Env,
			core.EnvVar{Name: "NEO4J_dbms_mode", Value: "CORE"},
			core.EnvVar{Name: "NEO4J_causal__clustering_discovery__type", Value: "DNS"},
			core.EnvVar{Name: "NEO4J_causal__clustering_initial__discovery__members", Value: discoveryMembers(instance)},
		)
	}
	for k, v := range instance.Spec.CoreArguments {
		templateSpec.Containers[0].Env = append(templateSpec.Containers[0].Env, core.EnvVar{Name: k, Value: v})
	}
	if instance.Spec.PersistentStorage == nil {
		templateSpec.Volumes = append(templateSpec.Volumes, core.Volume{
			Name:         "datadir",
			VolumeSource: core.VolumeSource{EmptyDir: &core.EmptyDirVolumeSource{},},
		})
	} else {
		storageSettings := instance.Spec.PersistentStorage
		volumeSize, _ := resource.ParseQuantity(storageSettings.Size)
		statefulSet.Spec.VolumeClaimTemplates = []core.PersistentVolumeClaim{
			{
				ObjectMeta: meta.ObjectMeta{
					Name:   "datadir",
					Labels: defaultLabels,
				},
				Spec: core.PersistentVolumeClaimSpec{
					AccessModes: []core.PersistentVolumeAccessMode{
						core.ReadWriteOnce,
					},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							core.ResourceStorage: volumeSize,
						},
					},
				},
			},
		}
		if storageSettings.StorageClass != "" {
			statefulSet.Spec.VolumeClaimTemplates[0].Spec.StorageClassName = &storageSettings.StorageClass
		}
	}
	return statefulSet, nil
}

func discoveryMembers(instance *database.Neo4jCluster) string {
	// We use DNS based discovery, so querying this address should return record A with
	// multiple IP addresses, one for each Noe4j Core service.
	// Example:
	// bash-4.4# nslookup neo4j-core-domain1
	// Address 1: 10.52.0.14 neo4j-core-domain1-1.neo4j-core-domain1.default.svc.cluster.local
	// Address 2: 10.52.1.15 neo4j-core-domain1-0.neo4j-core-domain1.default.svc.cluster.local
	// Address 3: 10.52.2.20 neo4j-core-domain1-2.neo4j-core-domain1.default.svc.cluster.local
	// Quote from Kubernetes services documentation:
	// "Headless" (without a cluster IP) Services are also assigned a DNS A record for a name
	// of the form my-svc.my-namespace.svc.cluster.local. Unlike normal Services, this resolves
	// to the set of IPs of the pods selected by the Service. Clients are expected to consume
	// the set or else use standard round-robin selection from the set.
	return fmt.Sprintf("%s:5000", instance.CoreServiceName())
}
