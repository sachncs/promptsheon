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
// GenerateDataKey on the supplied keyID and returns the
// plaintext form. The CiphertextBlob returned alongside is
// persisted by the Provider for later unwrapping via Decrypt.
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

// GenerateDataKeyWithCiphertextBlob returns both the plaintext
// and the wrapped CiphertextBlob. The Provider uses this on
// first-run to obtain a wrapped blob to persist in vault_state;
// subsequent runs Decrypt the persisted blob.
//
// This method is exposed via an optional interface (see
// provider.go's regenerateWrappedBlob) so adapters that don't
// return the wrapped blob (e.g. legacy or test doubles) cleanly
// fail closed instead of being silently wrong.
func (a *AWSKMSClient) GenerateDataKeyWithCiphertextBlob(ctx context.Context, keyID string) (plaintext []byte, ciphertext []byte, err error) {
	if keyID == "" {
		return nil, nil, fmt.Errorf("kmsbyok: empty keyID")
	}
	out, err := a.client.GenerateDataKey(ctx, &kms.GenerateDataKeyInput{
		KeyId:   &keyID,
		KeySpec: "AES_256",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("kmsbyok: aws generate data key: %w", err)
	}
	if out.Plaintext == nil {
		return nil, nil, fmt.Errorf("kmsbyok: aws returned nil plaintext")
	}
	if out.CiphertextBlob == nil {
		return nil, nil, fmt.Errorf("kmsbyok: aws returned nil CiphertextBlob")
	}
	return out.Plaintext, out.CiphertextBlob, nil
}

// Decrypt implements KMSClient. It unwraps the CiphertextBlob
// returned by GenerateDataKey back to the plaintext data key.
// The Provider calls this on cache miss (LRU size 16) so the
// process can survive KMS rotations without restart.
func (a *AWSKMSClient) Decrypt(ctx context.Context, ciphertextBlob []byte) ([]byte, error) {
	if len(ciphertextBlob) == 0 {
		return nil, fmt.Errorf("kmsbyok: empty ciphertextBlob")
	}
	out, err := a.client.Decrypt(ctx, &kms.DecryptInput{
		CiphertextBlob: ciphertextBlob,
	})
	if err != nil {
		return nil, fmt.Errorf("kmsbyok: aws decrypt: %w", err)
	}
	if out.Plaintext == nil {
		return nil, fmt.Errorf("kmsbyok: aws returned nil plaintext")
	}
	return out.Plaintext, nil
}
