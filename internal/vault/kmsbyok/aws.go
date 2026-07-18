package kmsbyok

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

// AWSKMSClient is the production adapter that wraps an AWS KMS
// client. It is concurrency-safe (the underlying SDK is).
type AWSKMSClient struct {
	client *kms.Client
}

// NewAWSKMSClient constructs an AWSKMSClient from the standard
// AWS SDK config (region, credentials resolved via the default
// credential chain: env, shared config, IAM role, etc.).
func NewAWSKMSClient(ctx context.Context, region string) (*AWSKMSClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("kmsbyok: load aws config: %w", err)
	}
	return &AWSKMSClient{client: kms.NewFromConfig(cfg)}, nil
}

// NewAWSKMSClientFromKMS constructs an adapter from an
// already-built *kms.Client. Tests use this to inject a custom
// transport; production paths use NewAWSKMSClient.
func NewAWSKMSClientFromKMS(client *kms.Client) *AWSKMSClient {
	return &AWSKMSClient{client: client}
}

// GenerateDataKey implements KMSClient. It calls
// GenerateDataKeyWithoutPlaintext on the supplied keyID and uses
// the wrapped CiphertextBlob as the opaque storage handle; the
// plaintext is fetched via Decrypt when the wrapping key is
// needed by Vault.
//
// Note: AWS KMS's standard GenerateDataKey returns the plaintext
// in the response, which is what callers want for envelope
// encryption. We pass it through verbatim.
//
// The ctx carries any per-call overrides (timeout, tracing, etc.).
func (a *AWSKMSClient) GenerateDataKey(ctx context.Context, keyID string) ([]byte, error) {
	if keyID == "" {
		return nil, fmt.Errorf("kmsbyok: empty keyID")
	}
	out, err := a.client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:   &keyID,
		KeySpec: "AES_256",
	})
	if err != nil {
		return nil, fmt.Errorf("kmsbyok: aws generate data key: %w", err)
	}
	if out.Plaintext == nil {
		return nil, fmt.Errorf("kmsbyok: aws returned nil plaintext")
	}
	return out.Plaintext, nil
}
