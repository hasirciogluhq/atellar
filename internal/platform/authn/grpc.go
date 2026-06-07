package authn

import (
	"context"
	"strings"

	"google.golang.org/grpc/metadata"
)

const metadataAuthorization = "authorization"

func ParseGRPCMetadata(md metadata.MD) (Credential, error) {
	values := md.Get(metadataAuthorization)
	if len(values) == 0 {
		return Credential{}, ErrMissingCredential
	}

	return ParseAuthorizationHeader(values[0])
}

func AuthenticateGRPC(ctx context.Context, authenticator Authenticator) (context.Context, *Principal, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, nil, ErrMissingCredential
	}

	credential, err := ParseGRPCMetadata(md)
	if err != nil {
		return ctx, nil, err
	}

	principal, err := authenticator.Authenticate(ctx, credential)
	if err != nil {
		return ctx, nil, err
	}

	return WithPrincipal(ctx, principal), principal, nil
}

func MetadataFromCredential(credential Credential) metadata.MD {
	return metadata.Pairs(metadataAuthorization, SchemeBearer+" "+strings.TrimSpace(credential.Value))
}

func OutgoingContext(ctx context.Context, credential Credential) context.Context {
	return metadata.NewOutgoingContext(ctx, MetadataFromCredential(credential))
}
