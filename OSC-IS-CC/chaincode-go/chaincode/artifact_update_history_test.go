package chaincode

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hyperledger/fabric-samples/asset-transfer-basic/chaincode-go/chaincode/mocks"
)

// Helper to create a valid stored artifact
func buildStoredArtifact() Artifact {
	hex64 := strings.Repeat("a", 64)
	return Artifact{
		ID:                "123e4567-e89b-12d3-a456-426614174000",
		Title:             "Valid Title",
		Description:       strings.Repeat("d", 60),
		Manifest:          []ManifestItem{{Algorithm: "sha256", Filename: "file.txt", Hash: hex64}},
		Footprint:         hex64,
		SubmitterEmail:    "user@example.com",
		SubmitterUsername: "tester",
	}
}

func newTxContext(stub *mocks.ChaincodeStub) contractapi.TransactionContextInterface {
	tx := &mocks.TransactionContext{}
	tx.GetStubReturns(stub)
	return tx
}

func TestUpdateArtifactDetails_Success(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stored := buildStoredArtifact()
	bStored, _ := json.Marshal(stored)
	stub.GetStateReturns(bStored, nil)

	var wroteKey string
	var wrote []byte
	stub.PutStateCalls(func(k string, v []byte) error { wroteKey = k; wrote = v; return nil })

	ctx := newTxContext(stub)
	upd := map[string]any{
		"keywords":         []string{"ai", "ml"},
		"links":            []string{"https://example.org/resource"},
		"dois":             []string{"10.1234/abcd.efg"},
		"fundingAgencies":  []string{"NSF"},
		"acknowledgements": "Thanks to the team.",
		"manifest":         []map[string]string{{"hash": strings.Repeat("b", 64), "filename": "file1.tif", "algorithm": "sha256"}},
		"footprint":        strings.Repeat("c", 64),
		"verified":         false,
		"submissionState":  "PENDING",
	}
	bUpd, _ := json.Marshal(upd)

	out, err := sc.UpdateArtifactDetails(ctx, stored.ID, string(bUpd))
	require.NoError(t, err)
	require.NotNil(t, out)
	require.Equal(t, "artifact:"+stored.ID, wroteKey)

	var saved Artifact
	require.NoError(t, json.Unmarshal(wrote, &saved))
	require.Equal(t, upd["keywords"], interface{}(saved.Keywords))
	require.Equal(t, upd["links"], interface{}(saved.Links))
	require.Equal(t, upd["dois"], interface{}(saved.Dois))
	require.Equal(t, upd["fundingAgencies"], interface{}(saved.FundingAgencies))
	require.Equal(t, upd["acknowledgements"], interface{}(saved.Acknowledgements))
	require.Equal(t, SubmissionState("PENDING"), saved.SubmissionState)
	// unchanged core fields
	require.Equal(t, stored.Title, saved.Title)
	require.Equal(t, stored.Description, saved.Description)
}

func TestUpdateArtifactDetails_RejectTitleDescription(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stored := buildStoredArtifact()
	bStored, _ := json.Marshal(stored)
	stub.GetStateReturns(bStored, nil)
	ctx := newTxContext(stub)

	// title
	_, err := sc.UpdateArtifactDetails(ctx, stored.ID, `{"title":"new"}`)
	require.Error(t, err)
	// description
	_, err = sc.UpdateArtifactDetails(ctx, stored.ID, `{"description":"new"}`)
	require.Error(t, err)
}

func TestUpdateArtifactDetails_UnknownFieldAndValidationErrors(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	stored := buildStoredArtifact()
	bStored, _ := json.Marshal(stored)
	stub.GetStateReturns(bStored, nil)
	ctx := newTxContext(stub)

	// unknown field
	_, err := sc.UpdateArtifactDetails(ctx, stored.ID, `{"bogus":1}`)
	require.Error(t, err)

	// bad URL
	_, err = sc.UpdateArtifactDetails(ctx, stored.ID, `{"links":["notaurl"]}`)
	require.Error(t, err)

	// bad DOI
	_, err = sc.UpdateArtifactDetails(ctx, stored.ID, `{"dois":["xyz"]}`)
	require.Error(t, err)

	// bad footprint
	_, err = sc.UpdateArtifactDetails(ctx, stored.ID, `{"footprint":"abc"}`)
	require.Error(t, err)

	// long acknowledgements
	_, err = sc.UpdateArtifactDetails(ctx, stored.ID, `{"acknowledgements":"`+strings.Repeat("x", 3001)+`"}`)
	require.Error(t, err)
}

func TestUpdateArtifactDetails_StateErrors(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	// read error
	stub.GetStateReturns(nil, errors.New("read error"))
	ctx := newTxContext(stub)
	_, err := sc.UpdateArtifactDetails(ctx, "123e4567-e89b-12d3-a456-426614174000", `{}`)
	require.Error(t, err)

	// not found
	stub = &mocks.ChaincodeStub{}
	stub.GetStateReturns(nil, nil)
	ctx = newTxContext(stub)
	_, err = sc.UpdateArtifactDetails(ctx, "123e4567-e89b-12d3-a456-426614174000", `{}`)
	require.Error(t, err)

	// write error
	stored := buildStoredArtifact()
	bStored, _ := json.Marshal(stored)
	stub = &mocks.ChaincodeStub{}
	stub.GetStateReturns(bStored, nil)
	stub.PutStateReturns(errors.New("write error"))
	ctx = newTxContext(stub)
	_, err = sc.UpdateArtifactDetails(ctx, stored.ID, `{}`)
	require.Error(t, err)
}

// Fake history iterator implementing shim.HistoryQueryIteratorInterface
type fakeHistoryIterator struct {
	items []*queryresult.KeyModification
	i     int
}

func (f *fakeHistoryIterator) Close() error  { return nil }
func (f *fakeHistoryIterator) HasNext() bool { return f.i < len(f.items) }
func (f *fakeHistoryIterator) Next() (*queryresult.KeyModification, error) {
	if !f.HasNext() {
		return nil, nil
	}
	item := f.items[f.i]
	f.i++
	return item, nil
}

func TestGetArtifactHistory_Success(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	ctx := newTxContext(stub)

	art1 := buildStoredArtifact()
	b1, _ := json.Marshal(art1)
	art2 := art1
	art2.Verified = true
	b2, _ := json.Marshal(art2)

	iter := &fakeHistoryIterator{items: []*queryresult.KeyModification{
		{TxId: "tx1", Timestamp: timestamppb.New(time.Unix(1000, 0)), IsDelete: false, Value: b1},
		{TxId: "tx2", Timestamp: timestamppb.New(time.Unix(2000, 0)), IsDelete: false, Value: b2},
	}}
	stub.GetHistoryForKeyReturns(iter, nil)

	entries, err := sc.GetArtifactHistory(ctx, art1.ID)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "tx1", entries[0].TxID)
	require.Equal(t, "tx2", entries[1].TxID)
	require.NotNil(t, entries[0].Value)
	require.True(t, entries[0].Timestamp.Unix() == 1000)
}

func TestGetArtifactHistory_DeleteAndErrors(t *testing.T) {
	sc := &SmartContract{}
	stub := &mocks.ChaincodeStub{}
	ctx := newTxContext(stub)

	// bad uuid
	_, err := sc.GetArtifactHistory(ctx, "not-a-uuid")
	require.Error(t, err)

	// read error
	stub = &mocks.ChaincodeStub{}
	stub.GetHistoryForKeyReturns(nil, errors.New("hist error"))
	ctx = newTxContext(stub)
	_, err = sc.GetArtifactHistory(ctx, "123e4567-e89b-12d3-a456-426614174000")
	require.Error(t, err)

	// delete record and invalid json record
	iter := &fakeHistoryIterator{items: []*queryresult.KeyModification{
		{TxId: "tx-del", Timestamp: timestamppb.New(time.Unix(1000, 0)), IsDelete: true, Value: nil},
		{TxId: "tx-bad", Timestamp: timestamppb.New(time.Unix(2000, 0)), IsDelete: false, Value: []byte("not-json")},
	}}
	stub = &mocks.ChaincodeStub{}
	stub.GetHistoryForKeyReturns(iter, nil)
	ctx = newTxContext(stub)
	entries, err := sc.GetArtifactHistory(ctx, "123e4567-e89b-12d3-a456-426614174000")
	require.Error(t, err)
	// ensure we progressed at least past delete without value error
	_ = entries
}
