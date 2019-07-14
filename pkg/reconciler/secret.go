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
	database "github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type Secret struct {
}

func (s *Secret) Create(instance *database.Neo4jCluster) (MetaObject, error) {
	if ! instance.Spec.AuthorizationEnabled() {
		return nil, nil
	}
	adminSecret, err := buildSecret(instance)
	if err != nil {
		return nil, err
	}
	return adminSecret, nil
}

func (s *Secret) Update(instance *database.Neo4jCluster, found runtime.Object) (MetaObject, bool, error) {
	other := found.(*core.Secret)
	return other, false, nil
}

func (s *Secret) GetName(instance *database.Neo4jCluster) string {
	return instance.SecretStoreName()
}

func (s *Secret) DefaultObject() runtime.Object {
	return &core.Secret{}
}

func buildSecret(instance *database.Neo4jCluster) (*core.Secret, error) {
	password, err := instance.Spec.AdminPasswordClearText()
	if err != nil {
		return nil, errors.NewBadRequest("password not base64 encoded")
	}
	return &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      instance.SecretStoreName(),
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"component": instance.LabelComponentName(),
			},
		},
		Type: core.SecretTypeOpaque,
		Data: map[string][]byte{"neo4j-password": []byte(*password)},
	}, nil
}
