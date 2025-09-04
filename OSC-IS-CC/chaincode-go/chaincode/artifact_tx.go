package chaincode

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

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
	doiPattern       = regexp.MustCompile(`^10\.\d{4,9}/[-._;()/:A-Za-z0-9]+$`)
)

const (
	linksCombinedMaxLen    = 2000
	keywordsCombinedMaxLen = 1000
	acknowledgementsMaxLen = 3000
)

// ArtifactUpdate represents the allowed updatable fields for an artifact.
// All fields are pointers to distinguish between omitted and provided values.
type ArtifactUpdate struct {
	Keywords         *[]string        `json:"keywords"`
	Links            *[]string        `json:"links"`
	Dois             *[]string        `json:"dois"`
	FundingAgencies  *[]string        `json:"fundingAgencies"`
	Acknowledgements *string          `json:"acknowledgements"`
	Manifest         *[]ManifestItem  `json:"manifest"`
	Footprint        *string          `json:"footprint"`
	SubmittedAt      *time.Time       `json:"submittedAt"`
	Verified         *bool            `json:"verified"`
	LastTimeVerified *time.Time       `json:"lastTimeVerified"`
	SubmissionState  *SubmissionState `json:"submissionState"`
}

func isValidHTTPURL(u string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(u))
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if parsed.Host == "" {
		return false
	}
	return true
}

func validateLinks(links []string) error {
	total := 0
	for _, l := range links {
		if !isValidHTTPURL(l) {
			return fmt.Errorf("links must be valid URLs (http/https): %s", l)
		}
		total += len(l)
		if total > linksCombinedMaxLen {
			return fmt.Errorf("combined links length must be ≤ %d characters", linksCombinedMaxLen)
		}
	}
	return nil
}

func validateDois(dois []string) error {
	for _, d := range dois {
		if !doiPattern.MatchString(strings.TrimSpace(d)) {
			return fmt.Errorf("invalid DOI format: %s", d)
		}
	}
	return nil
}

func validateKeywords(keywords []string) error {
	total := 0
	for _, k := range keywords {
		total += len(k)
		if total > keywordsCombinedMaxLen {
			return fmt.Errorf("combined keywords length must be ≤ %d characters", keywordsCombinedMaxLen)
		}
	}
	return nil
}

func validateManifestItems(manifest []ManifestItem) error {
	for _, m := range manifest {
		if strings.TrimSpace(m.Filename) == "" {
			return fmt.Errorf("manifest filename is required")
		}
		if strings.TrimSpace(m.Algorithm) == "" {
			return fmt.Errorf("manifest algorithm is required")
		}
		hash := strings.ToLower(strings.TrimSpace(m.Hash))
		if !sha256HexPattern.MatchString(hash) {
			return fmt.Errorf("manifest hash must be a 64-character lowercase hex string")
		}
	}
	return nil
}

func isValidSubmissionState(s SubmissionState) bool {
	return s == SubmissionStatePending || s == SubmissionStateFailed || s == SubmissionStateSuccess
}

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

// UpdateArtifactDetails updates allowed fields for an existing artifact.
// Disallowed fields: title, description. Unknown fields are rejected.
func (s *SmartContract) UpdateArtifactDetails(ctx contractapi.TransactionContextInterface, id string, updateJSON string) (*Artifact, error) {
	// Validate id
	id = strings.TrimSpace(strings.ToLower(id))
	if !uuidPattern.MatchString(id) {
		return nil, fmt.Errorf("id must be a UUID (lowercase, canonical form)")
	}

	key := "artifact:" + id
	existingBytes, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read world state: %w", err)
	}
	if existingBytes == nil {
		return nil, fmt.Errorf("artifact with id %s not found", id)
	}

	var current Artifact
	if err := json.Unmarshal(existingBytes, &current); err != nil {
		return nil, fmt.Errorf("failed to deserialize existing artifact: %w", err)
	}

	// Inspect raw keys to enforce forbidden/unknown fields
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(updateJSON), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if _, hasTitle := raw["title"]; hasTitle {
		return nil, fmt.Errorf("cannot update title or description")
	}
	if _, hasDescription := raw["description"]; hasDescription {
		return nil, fmt.Errorf("cannot update title or description")
	}

	allowed := map[string]bool{
		"keywords":         true,
		"links":            true,
		"dois":             true,
		"fundingAgencies":  true,
		"acknowledgements": true,
		"manifest":         true,
		"footprint":        true,
		"submittedAt":      true,
		"verified":         true,
		"lastTimeVerified": true,
		"submissionState":  true,
	}
	for k := range raw {
		if !allowed[k] {
			return nil, fmt.Errorf("unknown field in update: %s", k)
		}
	}

	// Decode into typed update struct
	var upd ArtifactUpdate
	if err := json.Unmarshal([]byte(updateJSON), &upd); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	updated := current

	// Merge fields if provided
	if upd.Keywords != nil {
		updated.Keywords = *upd.Keywords
	}
	if upd.Links != nil {
		updated.Links = *upd.Links
	}
	if upd.Dois != nil {
		updated.Dois = *upd.Dois
	}
	if upd.FundingAgencies != nil {
		updated.FundingAgencies = *upd.FundingAgencies
	}
	if upd.Acknowledgements != nil {
		updated.Acknowledgements = strings.TrimSpace(*upd.Acknowledgements)
	}
	if upd.Manifest != nil {
		updated.Manifest = *upd.Manifest
	}
	if upd.Footprint != nil {
		updated.Footprint = strings.TrimSpace(strings.ToLower(*upd.Footprint))
	}
	if upd.SubmittedAt != nil {
		updated.SubmittedAt = upd.SubmittedAt
	}
	if upd.Verified != nil {
		updated.Verified = *upd.Verified
	}
	if upd.LastTimeVerified != nil {
		updated.LastTimeVerified = upd.LastTimeVerified
	}
	if upd.SubmissionState != nil {
		if !isValidSubmissionState(*upd.SubmissionState) {
			return nil, fmt.Errorf("submissionState must be one of PENDING, FAILED, SUCCESS")
		}
		updated.SubmissionState = *upd.SubmissionState
	}

	// Ensure slices are non-nil for deterministic JSON
	if updated.Dois == nil {
		updated.Dois = []string{}
	}
	if updated.FundingAgencies == nil {
		updated.FundingAgencies = []string{}
	}
	if updated.Keywords == nil {
		updated.Keywords = []string{}
	}
	if updated.Links == nil {
		updated.Links = []string{}
	}

	// Re-run create-time validations on the merged artifact
	if l := len(strings.TrimSpace(updated.Title)); l < 3 || l > 200 {
		return nil, fmt.Errorf("title must be between 3 and 200 characters")
	}
	if l := len(strings.TrimSpace(updated.Description)); l < 50 || l > 3000 {
		return nil, fmt.Errorf("description must be between 50 and 3000 characters")
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(updated.SubmitterEmail)); err != nil {
		return nil, fmt.Errorf("submitterEmail must be a valid email address")
	}
	if strings.TrimSpace(updated.SubmitterUsername) == "" {
		return nil, fmt.Errorf("submitterUsername is required")
	}
	if !sha256HexPattern.MatchString(updated.Footprint) {
		return nil, fmt.Errorf("footprint must be a 64-character lowercase hex string")
	}

	// Apply update-specific validations
	if err := validateLinks(updated.Links); err != nil {
		return nil, err
	}
	if err := validateKeywords(updated.Keywords); err != nil {
		return nil, err
	}
	if err := validateDois(updated.Dois); err != nil {
		return nil, err
	}
	if updated.Acknowledgements != "" && len(updated.Acknowledgements) > acknowledgementsMaxLen {
		return nil, fmt.Errorf("acknowledgements must be ≤ %d characters", acknowledgementsMaxLen)
	}
	if err := validateManifestItems(updated.Manifest); err != nil {
		return nil, err
	}

	// Persist
	outBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize artifact: %w", err)
	}
	if err := ctx.GetStub().PutState(key, outBytes); err != nil {
		return nil, fmt.Errorf("failed to write artifact: %w", err)
	}

	return &updated, nil
}

// GetArtifactHistory returns the modification history for an artifact.
func (s *SmartContract) GetArtifactHistory(ctx contractapi.TransactionContextInterface, id string) ([]*struct {
	TxID      string    `json:"txId"`
	Timestamp time.Time `json:"timestamp"`
	IsDelete  bool      `json:"isDelete"`
	Value     *Artifact `json:"value,omitempty"`
}, error) {
	// Validate id
	id = strings.TrimSpace(strings.ToLower(id))
	if !uuidPattern.MatchString(id) {
		return nil, fmt.Errorf("id must be a UUID (lowercase, canonical form)")
	}

	key := "artifact:" + id
	iterator, err := ctx.GetStub().GetHistoryForKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read history: %w", err)
	}
	defer iterator.Close()

	entries := make([]*struct {
		TxID      string    `json:"txId"`
		Timestamp time.Time `json:"timestamp"`
		IsDelete  bool      `json:"isDelete"`
		Value     *Artifact `json:"value,omitempty"`
	}, 0)

	for iterator.HasNext() {
		mod, err := iterator.Next()
		if err != nil {
			return nil, fmt.Errorf("history iterator error: %w", err)
		}

		ts := time.Unix(mod.Timestamp.Seconds, int64(mod.Timestamp.Nanos))
		entry := &struct {
			TxID      string    `json:"txId"`
			Timestamp time.Time `json:"timestamp"`
			IsDelete  bool      `json:"isDelete"`
			Value     *Artifact `json:"value,omitempty"`
		}{TxID: mod.TxId, Timestamp: ts, IsDelete: mod.IsDelete}

		if !mod.IsDelete && len(mod.Value) > 0 {
			var a Artifact
			if err := json.Unmarshal(mod.Value, &a); err != nil {
				return nil, fmt.Errorf("failed to deserialize history value: %w", err)
			}
			entry.Value = &a
		}
		entries = append(entries, entry)
	}

	return entries, nil
}
