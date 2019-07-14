package controller

import (
	"github.com/lukasz-antoniak/neo4j-operator/pkg/controller/neo4jcluster"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, neo4jcluster.Add)
}
