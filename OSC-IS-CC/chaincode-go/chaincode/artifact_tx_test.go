package chaincode

import (
    "encoding/json"
    "errors"
    "strings"
    "testing"

    "github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
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
    require.NotNil(t, stored.Verified)
    require.Equal(t, false, *stored.Verified)
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


