package adapters

import (
	"chainlink/core/logger"
	strpkg "chainlink/core/store"
	"chainlink/core/store/models"
	"encoding/hex"

	ontcommon "github.com/ontio/ontology/common"
	"gopkg.in/guregu/null.v3"
)

const (
	FulfillOracleRequest = "fulfillOracleRequest"
)

// OntTx holds the Address to send the result to and the callbackAddress, callbackFunction
// to execute.
type OntTx struct {
	Address          string `json:"address"`
	RequestID        string `json:"requestID"`
	Payment          string `json:"payment"`
	CallbackAddress  string `json:"callbackAddress"`
	CallbackFunction string `json:"callbackFunction"`
	Expiration       string `json:"expiration"`
}

// TaskType returns the type of Adapter.
func (e *OntTx) TaskType() models.TaskType {
	return TaskTypeOntTx
}

// Perform creates the run result for the transaction
func (otx *OntTx) Perform(input models.RunInput, store *strpkg.Store) models.RunOutput {
	data := input.Result().String()
	requestID, err := hex.DecodeString(otx.RequestID)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	payment, err := hex.DecodeString(otx.Payment)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	callbackAddress, err := hex.DecodeString(otx.CallbackAddress)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	callbackFunction, err := hex.DecodeString(otx.CallbackFunction)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	expiration, err := hex.DecodeString(otx.Expiration)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	args := []interface{}{FulfillOracleRequest, []interface{}{store.OntTxManager.Account().Address[:],
		requestID, payment, callbackAddress, callbackFunction, expiration, data}}
	address, err := ontcommon.AddressFromHexString(otx.Address)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	ontTx, err := store.OntTxManager.InvokeNeoVm(null.StringFrom(input.JobRunID().String()), address, args)
	if err != nil {
		return models.NewRunOutputError(err)
	}

	output, err := models.JSON{}.Add("result", ontTx.Hash)
	if err != nil {
		return models.NewRunOutputError(err)
	}
	logger.Debugw("ontTx success", "txHash", ontTx.Hash)
	return models.NewRunOutputComplete(output)
}
