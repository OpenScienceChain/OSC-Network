package chaincode

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric-samples/asset-transfer-basic/chaincode-go/chaincode/mocks"
)

func buildValidArtifact() Artifact {
	title := "Valid Title"
	description := strings.Repeat("d", 60)
	uuid := "123e4567-e89b-12d3-a456-426614174000"
	hex64 := strings.Repeat("a", 64)
	manifest := []ManifestItem{{Algorithm: "sha256", Filename: "file.txt", Hash: hex64}}
	return Artifact{
		ID:                uuid,
		Title:             title,
		Description:       description,
		Manifest:          manifest,
		Footprint:         hex64,
		SubmitterEmail:    "user@example.com",
		SubmitterUsername: "tester",
	}
}

func newContextWithStub(stub *mocks.ChaincodeStub) contractapi.TransactionContextInterface {
	tx := &mocks.TransactionContext{}
	tx.GetStubReturns(stub)
	return tx
}

func TestCreateArtifact_Success_DefaultsAndState(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	// no existing key
	stub.GetStateReturns(nil, nil)

	var wroteKey string
	var wrote []byte
	stub.PutStateCalls(func(k string, v []byte) error { wroteKey = k; wrote = v; return nil })

	art := buildValidArtifact()
	payload, _ := json.Marshal(art)

	ctx := newContextWithStub(stub)
	out, err := sc.CreateArtifact(ctx, string(payload))
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, "artifact:"+strings.ToLower(art.ID), wroteKey)

	var stored Artifact
	require.NoError(t, json.Unmarshal(wrote, &stored))
	// verified defaults to false
	require.Equal(t, false, stored.Verified)
	require.Equal(t, SubmissionStateSuccess, stored.SubmissionState)
}

func TestCreateArtifact_InvalidJSON(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	ctx := newContextWithStub(stub)
	_, err := sc.CreateArtifact(ctx, "{bad json}")
	require.Error(t, err)
}

func TestCreateArtifact_InvalidUUID(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stub.GetStateReturns(nil, nil)
	ctx := newContextWithStub(stub)

	art := buildValidArtifact()
	art.ID = "not-a-uuid"
	payload, _ := json.Marshal(art)
	_, err := sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)
}

func TestCreateArtifact_TitleAndDescriptionLengths(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stub.GetStateReturns(nil, nil)
	ctx := newContextWithStub(stub)

	art := buildValidArtifact()
	art.Title = "ab"
	payload, _ := json.Marshal(art)
	_, err := sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)

	art = buildValidArtifact()
	art.Description = "too short"
	payload, _ = json.Marshal(art)
	_, err = sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)
}

func TestCreateArtifact_EmailUsernameFootprintValidation(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stub.GetStateReturns(nil, nil)
	ctx := newContextWithStub(stub)

	art := buildValidArtifact()
	art.SubmitterEmail = "invalid-email"
	payload, _ := json.Marshal(art)
	_, err := sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)

	art = buildValidArtifact()
	art.SubmitterUsername = ""
	payload, _ = json.Marshal(art)
	_, err = sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)

	art = buildValidArtifact()
	art.Footprint = "abc"
	payload, _ = json.Marshal(art)
	_, err = sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)
}

func TestCreateArtifact_KeyAlreadyExists(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	// simulate existing state
	stub.GetStateReturns([]byte("{}"), nil)
	ctx := newContextWithStub(stub)

	art := buildValidArtifact()
	payload, _ := json.Marshal(art)
	_, err := sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)
}

func TestCreateArtifact_GetStateErrorAndPutStateError(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	// read error path
	stub.GetStateReturns(nil, errors.New("read error"))
	ctx := newContextWithStub(stub)

	art := buildValidArtifact()
	payload, _ := json.Marshal(art)
	_, err := sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)

	// success read, but write error path
	stub = &mocks.ChaincodeStub{}
	stub.GetStateReturns(nil, nil)
	stub.PutStateReturns(errors.New("write error"))
	ctx = newContextWithStub(stub)
	_, err = sc.CreateArtifact(ctx, string(payload))
	require.Error(t, err)
}

func TestListArtifacts_Empty(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	iter := &mocks.StateQueryIterator{}
	iter.HasNextReturns(false)
	stub.GetStateByRangeReturns(iter, nil)

	ctx := newContextWithStub(stub)
	out, err := sc.ListArtifacts(ctx)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Len(t, out, 0)
}

func TestListArtifacts_ReturnsAll(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	iter := &mocks.StateQueryIterator{}

	a1 := buildValidArtifact()
	a1.ID = "123e4567-e89b-12d3-a456-426614174000"
	b1, _ := json.Marshal(a1)
	a2 := buildValidArtifact()
	a2.ID = "223e4567-e89b-12d3-a456-426614174000"
	b2, _ := json.Marshal(a2)

	iter.HasNextReturnsOnCall(0, true)
	iter.NextReturnsOnCall(0, &queryresult.KV{Key: "artifact:" + a1.ID, Value: b1}, nil)
	iter.HasNextReturnsOnCall(1, true)
	iter.NextReturnsOnCall(1, &queryresult.KV{Key: "artifact:" + a2.ID, Value: b2}, nil)
	iter.HasNextReturnsOnCall(2, false)

	stub.GetStateByRangeReturns(iter, nil)
	ctx := newContextWithStub(stub)

	out, err := sc.ListArtifacts(ctx)
	require.NoError(t, err)
	require.Len(t, out, 2)
	require.Equal(t, a1.ID, out[0].ID)
	require.Equal(t, a2.ID, out[1].ID)
}

func TestListArtifacts_IteratorError(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	iter := &mocks.StateQueryIterator{}

	iter.HasNextReturns(true)
	iter.NextReturns(nil, errors.New("iterator error"))
	stub.GetStateByRangeReturns(iter, nil)

	ctx := newContextWithStub(stub)
	_, err := sc.ListArtifacts(ctx)
	require.Error(t, err)
}

func TestListArtifacts_GetStateByRangeError(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stub.GetStateByRangeReturns(nil, errors.New("range error"))
	ctx := newContextWithStub(stub)
	_, err := sc.ListArtifacts(ctx)
	require.Error(t, err)
}

func TestListArtifacts_InvalidJSON(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	iter := &mocks.StateQueryIterator{}

	iter.HasNextReturnsOnCall(0, true)
	iter.NextReturnsOnCall(0, &queryresult.KV{Key: "artifact:bad", Value: []byte("not-json")}, nil)
	stub.GetStateByRangeReturns(iter, nil)

	ctx := newContextWithStub(stub)
	_, err := sc.ListArtifacts(ctx)
	require.Error(t, err)
}
