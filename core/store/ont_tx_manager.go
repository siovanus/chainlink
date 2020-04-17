package store

import (
	"chainlink/core/store/models"
	"chainlink/core/store/orm"
	"encoding/hex"

	ontology_go_sdk "github.com/ontio/ontology-go-sdk"
	sdkcom "github.com/ontio/ontology-go-sdk/common"
	ontcommon "github.com/ontio/ontology/common"
	"gopkg.in/guregu/null.v3"
)

const (
	DefaultOntGasPrice = 500
)

// OntTxManager contains fields for the Ontology client, the KeyStore,
// the local Config for the application, and the database.
type OntTxManager struct {
	sdk     *ontology_go_sdk.OntologySdk
	account *ontology_go_sdk.Account
	config  orm.ConfigReader
	orm     *orm.ORM
}

// NewOntTxManager constructs an OntTxManager using the passed variables and
// initializing internal variables.
func NewOntTxManager(sdk *ontology_go_sdk.OntologySdk, account *ontology_go_sdk.Account, config orm.ConfigReader, orm *orm.ORM) *OntTxManager {
	return &OntTxManager{
		sdk:     sdk,
		account: account,
		config:  config,
		orm:     orm,
	}
}

func (otm *OntTxManager) Account() *ontology_go_sdk.Account {
	return otm.account
}

func (otm *OntTxManager) InvokeNeoVm(surrogateID null.String, address ontcommon.Address,
	args []interface{}) (*models.OntTx, error) {
	tx, err := otm.sdk.NeoVM.NewNeoVMInvokeTransaction(DefaultOntGasPrice, otm.config.OntGasLimit(),
		address, args)
	if err != nil {
		return nil, err
	}
	err = otm.sdk.SignToTransaction(tx, otm.account)
	if err != nil {
		return nil, err
	}
	txHash, err := otm.sdk.SendTransaction(tx)
	if err != nil {
		return nil, err
	}
	imTx, err := tx.IntoImmutable()
	if err != nil {
		return nil, err
	}
	ontTx := &models.OntTx{
		SurrogateID: surrogateID,
		From:        otm.account.Address.ToBase58(),
		To:          address.ToHexString(),
		GasPrice:    DefaultOntGasPrice,
		GasLimit:    otm.config.OntGasLimit(),
		Hash:        txHash.ToHexString(),
		SignedRawTx: hex.EncodeToString(imTx.ToArray()),
	}
	ontTx, err = otm.orm.CreateOntTx(ontTx)
	if err != nil {
		return nil, err
	}
	return ontTx, nil
}

func (otm *OntTxManager) GetCurrentBlockHeight() (uint32, error) {
	return otm.sdk.GetCurrentBlockHeight()
}

func (otm *OntTxManager) GetSmartContractEventByBlock(height uint32) ([]*sdkcom.SmartContactEvent, error) {
	return otm.sdk.GetSmartContractEventByBlock(height)
}
