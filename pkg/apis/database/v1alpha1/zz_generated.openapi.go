// +build !ignore_autogenerated

// Code generated by openapi-gen. DO NOT EDIT.

// This file was autogenerated by openapi-gen. Do not edit it manually!

package v1alpha1

import (
	spec "github.com/go-openapi/spec"
	common "k8s.io/kube-openapi/pkg/common"
)

func GetOpenAPIDefinitions(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition {
	return map[string]common.OpenAPIDefinition{
		"github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jCluster":       schema_pkg_apis_database_v1alpha1_Neo4jCluster(ref),
		"github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jClusterSpec":   schema_pkg_apis_database_v1alpha1_Neo4jClusterSpec(ref),
		"github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jClusterStatus": schema_pkg_apis_database_v1alpha1_Neo4jClusterStatus(ref),
	}
}

func schema_pkg_apis_database_v1alpha1_Neo4jCluster(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "Neo4jCluster is the Schema for the neo4jclusters API",
				Properties: map[string]spec.Schema{
					"kind": {
						SchemaProps: spec.SchemaProps{
							Description: "Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"apiVersion": {
						SchemaProps: spec.SchemaProps{
							Description: "APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#resources",
							Type:        []string{"string"},
							Format:      "",
						},
					},
					"metadata": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"),
						},
					},
					"spec": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jClusterSpec"),
						},
					},
					"status": {
						SchemaProps: spec.SchemaProps{
							Ref: ref("github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jClusterStatus"),
						},
					},
				},
			},
		},
		Dependencies: []string{
			"github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jClusterSpec", "github.com/lukasz-antoniak/neo4j-operator/pkg/apis/database/v1alpha1.Neo4jClusterStatus", "k8s.io/apimachinery/pkg/apis/meta/v1.ObjectMeta"},
	}
}

func schema_pkg_apis_database_v1alpha1_Neo4jClusterSpec(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "Neo4jClusterSpec defines the desired state of Neo4jCluster",
				Properties:  map[string]spec.Schema{},
			},
		},
		Dependencies: []string{},
	}
}

func schema_pkg_apis_database_v1alpha1_Neo4jClusterStatus(ref common.ReferenceCallback) common.OpenAPIDefinition {
	return common.OpenAPIDefinition{
		Schema: spec.Schema{
			SchemaProps: spec.SchemaProps{
				Description: "Neo4jClusterStatus defines the observed state of Neo4jCluster",
				Properties:  map[string]spec.Schema{},
			},
		},
		Dependencies: []string{},
	}
}