package web

import (
	"net/http"

	"chainlink/core/services/chainlink"
	"chainlink/core/store/orm"
	"chainlink/core/store/presenters"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// TransactionsController displays ontology transactions requests.
type OntTransactionsController struct {
	App chainlink.Application
}

// OntIndex returns paginated ontology transaction attempts
func (otc *OntTransactionsController) Index(c *gin.Context, size, page, offset int) {
	txs, count, err := otc.App.GetStore().OntTransactions(offset, size)
	ptxs := make([]presenters.OntTx, len(txs))
	for i, tx := range txs {
		txp := presenters.NewOntTx(&tx)
		ptxs[i] = txp
	}
	paginatedResponse(c, "OntTransactions", size, page, ptxs, count, err)
}

// Show returns the details of a ontology Transasction details.
// Example:
//  "<application>/ont_transactions/:TxHash"
func (otc *OntTransactionsController) Show(c *gin.Context) {
	hash := common.HexToHash(c.Param("TxHash"))

	ontTx, err := otc.App.GetStore().FindOntTx(hash)
	if errors.Cause(err) == orm.ErrorNotFound {
		jsonAPIError(c, http.StatusNotFound, errors.New("Transaction not found"))
		return
	}
	if err != nil {
		jsonAPIError(c, http.StatusInternalServerError, err)
		return
	}

	jsonAPIResponse(c, presenters.NewOntTx(ontTx), "ont_transaction")
}
