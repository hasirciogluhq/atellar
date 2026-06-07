package authn

import (
	"context"

	"google.golang.org/grpc/metadata"
)

func ParseGRPCMetadataFromContext(ctx context.Context) (Credential, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return Credential{}, ErrMissingCredential
	}

	return ParseGRPCMetadata(md)
}
