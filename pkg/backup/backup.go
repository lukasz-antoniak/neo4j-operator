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

package backup

import (
	"fmt"
	"github.com/go-logr/logr"
	database "github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ScheduleBackup(client *kubernetes.Clientset, logger logr.Logger, instance *database.Neo4jCluster) error {
	// TODO(lantonia): Test backup and recovery.
	if instance.Spec.Backup == nil {
		return nil
	}

	jobName := backupJobName(instance)
	volumeName := backupVolumeName(instance)
	defaultLabels := map[string]string{
		"component": instance.LabelComponentName(),
		"role":      "neo4j-backup",
	}

	volume, err := client.CoreV1().PersistentVolumeClaims(instance.Namespace).Get(volumeName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			volumeSize, _ := resource.ParseQuantity(instance.Spec.Backup.Size)
			volume = &core.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:   volumeName,
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
			}
			if instance.Spec.Backup.StorageClass != "" {
				volume.Spec.StorageClassName = &instance.Spec.Backup.StorageClass
			}
			logger.Info(fmt.Sprintf("Creating new %T", volume), "Namespace", volume.GetNamespace(), "Name", volume.GetName())
			if _, err := client.CoreV1().PersistentVolumeClaims(instance.Namespace).Create(volume); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	imagePullPolicy := instance.Spec.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = "IfNotPresent"
	}
	limitCpu, _ := resource.ParseQuantity(instance.Spec.Resources.Limits.CPU)
	limitMemory, _ := resource.ParseQuantity(instance.Spec.Resources.Limits.Memory)
	requestCpu, _ := resource.ParseQuantity(instance.Spec.Resources.Requests.CPU)
	requestMemory, _ := resource.ParseQuantity(instance.Spec.Resources.Requests.Memory)

	job, err := client.BatchV1beta1().CronJobs(instance.Namespace).Get(jobName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			job = &v1beta1.CronJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:   jobName,
					Labels: defaultLabels,
				},
				Spec: v1beta1.CronJobSpec{
					Schedule: instance.Spec.Backup.Schedule,
					ConcurrencyPolicy: v1beta1.ForbidConcurrent,
					JobTemplate: v1beta1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: core.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: defaultLabels,
								},
								Spec: core.PodSpec{
									RestartPolicy: "OnFailure",
									Containers: []core.Container{
										{
											Name:            "backup",
											Image:           instance.Spec.DockerImage(),
											ImagePullPolicy: core.PullPolicy(imagePullPolicy),
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
											Command: []string{
												"/bin/bash",
												"-c",
												fmt.Sprintf("exec /var/lib/neo4j/bin/neo4j-admin backup --from=%s:6362 --backup-dir=/backup --name=graph.db-backup", instance.RandomCorePod()),
											},
											VolumeMounts: []core.VolumeMount{
												{ Name: "backupdir", MountPath: "/backup", ReadOnly: false },
											},
										},
									},
									Volumes: []core.Volume{
										{
											Name:         "backupdir",
											VolumeSource: core.VolumeSource{
												PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{ClaimName: volumeName, ReadOnly: false},
											},
										},
									},
								},
							},
						},
					},
				},
			}
			logger.Info(fmt.Sprintf("Creating new %T", job), "Namespace", job.GetNamespace(), "Name", job.GetName())
			if _, err := client.BatchV1beta1().CronJobs(instance.Namespace).Create(job); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	return nil
}

func backupJobName(instance *database.Neo4jCluster) string {
	return fmt.Sprintf("neo4j-backup-%s", instance.Name)
}

func backupVolumeName(instance *database.Neo4jCluster) string {
	return fmt.Sprintf("neo4j-backup-%s", instance.Name)
}