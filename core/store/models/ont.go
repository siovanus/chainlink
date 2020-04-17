package models

import (
	"fmt"
	"math/big"

	"gopkg.in/guregu/null.v3"
)

type OntHeight struct {
	Key    string `gorm:"primary_key"`
	Height uint32 `gorm:"not null"`
}

func NewOntHeight(height uint32) *OntHeight {
	return &OntHeight{
		Key:    "TrackHeight",
		Height: height,
	}
}

// String returns a string representation of this height.
func (h *OntHeight) String() string {
	height := big.NewInt(int64(h.Height))
	return height.String()
}

// Tx contains fields necessary for an Ontology transaction
type OntTx struct {
	ID uint64 `gorm:"primary_key;auto_increment"`

	// SurrogateID is used to look up a transaction using a secondary ID, used to
	// associate jobs with transactions so that we don't double spend in certain
	// failure scenarios
	SurrogateID null.String `gorm:"index;unique"`

	From string `gorm:"index;not null"`
	To   string `gorm:"not null"`

	GasPrice uint64 `gorm:"not null"`
	GasLimit uint64 `gorm:"not null"`

	Hash        string `gorm:"not null"`
	SignedRawTx string `gorm:"type:text;not null"`
}

// String implements Stringer for Tx
func (ot *OntTx) String() string {
	return fmt.Sprintf("Tx(ID: %d, From: %s, To: %s, Hash: %s)",
		ot.ID,
		ot.From,
		ot.To,
		ot.Hash)
}
