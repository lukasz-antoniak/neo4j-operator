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
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type MetaObject interface {
	meta.Object
	runtime.Object
}

type ManagedObject interface {
	Create(instance *database.Neo4jCluster) (MetaObject, error)
	Update(instance *database.Neo4jCluster, found runtime.Object) (MetaObject, bool, error)
	GetName(instance *database.Neo4jCluster) string
	DefaultObject() runtime.Object
}
