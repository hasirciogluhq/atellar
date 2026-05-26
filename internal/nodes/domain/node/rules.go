package node

import "strings"

type NodeResolutionIdentifierType string

const (
	NodeResolutionIdentifierTypeID   NodeResolutionIdentifierType = "id"
	NodeResolutionIdentifierTypeName NodeResolutionIdentifierType = "name"
)

func ResolveNodeResolutionIdentifierContext(identifier string) (NodeResolutionIdentifierType, error) {
	if strings.HasPrefix(identifier, "node_") {
		return NodeResolutionIdentifierTypeID, nil
	}

	return NodeResolutionIdentifierTypeName, nil
}
