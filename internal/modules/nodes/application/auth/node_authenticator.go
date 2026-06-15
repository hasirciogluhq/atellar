package auth

import (
	"context"

	"github.com/hasirciogluhq/atellar/internal/modules/nodes/ports"
	"github.com/hasirciogluhq/atellar/internal/platform/authn"
	"github.com/hasirciogluhq/atellar/internal/platform/authz"
)

type NodeAuthenticator struct {
	nodeRepository ports.NodeRepositoryInterface
}

func NewNodeAuthenticator(nodeRepository ports.NodeRepositoryInterface) *NodeAuthenticator {
	return &NodeAuthenticator{nodeRepository: nodeRepository}
}

func (a *NodeAuthenticator) Authenticate(ctx context.Context, credential authn.Credential) (*authn.Principal, error) {
	if credential.Type != authn.CredentialTypeNodeAPIKey {
		return nil, authn.ErrInvalidCredential
	}

	authenticatedNode, err := a.nodeRepository.AuthenticateNodeByAPIKey(ctx, credential.Value)
	if err != nil {
		return nil, err
	}

	if authenticatedNode == nil {
		return nil, authn.ErrInvalidCredential
	}

	return &authn.Principal{
		SubjectType: authn.SubjectTypeNode,
		Node: &authn.Node{
			ID:   authenticatedNode.ID,
			Name: authenticatedNode.Name,
		},
		Scopes: authz.NodeAgentScopes(),
	}, nil
}
