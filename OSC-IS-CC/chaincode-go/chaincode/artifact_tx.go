package chaincode

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"regexp"
	"strings"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

// SmartContract provides functions for managing an Asset
type SmartContract struct {
	contractapi.Contract
}

// Constants for artifact submission states
const (
	SubmissionStatePending SubmissionState = "PENDING"
	SubmissionStateFailed  SubmissionState = "FAILED"
	SubmissionStateSuccess SubmissionState = "SUCCESS"
)

var (
	uuidPattern      = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
	sha256HexPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// CreateArtifact stores a new artifact on the ledger. The input must be a JSON string
// matching the Artifact struct shape. The function validates core constraints and
// writes the artifact under key "artifact:{id}". On success, SubmissionState is set to SUCCESS
// and Verified defaults to false when not provided.
func (s *SmartContract) CreateArtifact(ctx contractapi.TransactionContextInterface, artifactJSON string) (*Artifact, error) {
	// Decode
	var artifact Artifact
	if err := json.Unmarshal([]byte(artifactJSON), &artifact); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate basic fields
	artifact.ID = strings.TrimSpace(strings.ToLower(artifact.ID))
	if !uuidPattern.MatchString(artifact.ID) {
		return nil, fmt.Errorf("id must be a UUID (lowercase, canonical form)")
	}

	if l := len(strings.TrimSpace(artifact.Title)); l < 3 || l > 200 {
		return nil, fmt.Errorf("title must be between 3 and 200 characters")
	}

	if l := len(strings.TrimSpace(artifact.Description)); l < 50 || l > 3000 {
		return nil, fmt.Errorf("description must be between 50 and 3000 characters")
	}

	// Email validation (minimal but robust using stdlib)
	if _, err := mail.ParseAddress(strings.TrimSpace(artifact.SubmitterEmail)); err != nil {
		return nil, fmt.Errorf("submitterEmail must be a valid email address")
	}
	if strings.TrimSpace(artifact.SubmitterUsername) == "" {
		return nil, fmt.Errorf("submitterUsername is required")
	}

	// Footprint must be a 64-char lowercase hex string
	artifact.Footprint = strings.TrimSpace(strings.ToLower(artifact.Footprint))
	if !sha256HexPattern.MatchString(artifact.Footprint) {
		return nil, fmt.Errorf("footprint must be a 64-character lowercase hex string")
	}

	// Default verified to false if not provided
	// Default verified to false if not provided (zero-value already false). Nothing to do.
	if artifact.Dois == nil {
		artifact.Dois = []string{}
	}
	if artifact.FundingAgencies == nil {
		artifact.FundingAgencies = []string{}
	}
	if artifact.Keywords == nil {
		artifact.Keywords = []string{}
	}
	if artifact.Links == nil {
		artifact.Links = []string{}
	}

	// Ensure key uniqueness
	key := "artifact:" + artifact.ID
	existing, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read world state: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("artifact with id %s already exists", artifact.ID)
	}

	// On success we mark the submission as SUCCESS. Note: if this function returns an error,
	// the transaction is not committed, so FAILED would not be recorded on-chain.
	artifact.SubmissionState = SubmissionStateSuccess

	// Persist
	bytes, err := json.Marshal(artifact)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize artifact: %w", err)
	}
	if err := ctx.GetStub().PutState(key, bytes); err != nil {
		return nil, fmt.Errorf("failed to write artifact: %w", err)
	}

	return &artifact, nil
}

// ListArtifacts returns all artifacts stored under keys with the "artifact:" prefix.
// This is a simple full-scan by prefix with no pagination.
func (s *SmartContract) ListArtifacts(ctx contractapi.TransactionContextInterface) ([]*Artifact, error) {
	// Scan only keys starting with "artifact:" using a bounded range
	startKey := "artifact:"
	// Use the next ASCII character after ':' to bound the range
	endKey := "artifact;"

	iterator, err := ctx.GetStub().GetStateByRange(startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to read world state: %w", err)
	}
	defer iterator.Close()

	artifacts := make([]*Artifact, 0)
	for iterator.HasNext() {
		kv, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("iterator error: %w", err)
		}

		var artifact Artifact
		if err := json.Unmarshal(kv.Value, &artifact); err != nil {
			return nil, fmt.Errorf("failed to deserialize artifact with key %s: %w", kv.Key, err)
		}
		artifacts = append(artifacts, &artifact)
	}

	return artifacts, nil
}
