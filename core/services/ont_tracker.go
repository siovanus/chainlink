package services

import (
	"chainlink/core/assets"
	"chainlink/core/logger"
	"chainlink/core/store"
	"chainlink/core/store/models"
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/multierr"
	"sync"
	"time"

	ontcommon "github.com/ontio/ontology/common"
)

type OntTracker struct {
	store                 *store.Store
	runManager            RunManager
	ontTxManager          *store.OntTxManager
	mutex                 *sync.RWMutex
	jobSubscriptions      map[string]models.JobSpec
	contractSubscriptions map[string]map[string]models.Initiator
	height                uint32
	quit                  chan struct{}
}

// NewOntTracker returns a new ont tracker.
func NewOntTracker(store *store.Store, runManager RunManager, ontTxManager *store.OntTxManager) *OntTracker {
	ot := &OntTracker{
		store:                 store,
		runManager:            runManager,
		ontTxManager:          ontTxManager,
		mutex:                 &sync.RWMutex{},
		jobSubscriptions:      map[string]models.JobSpec{},
		contractSubscriptions: map[string]map[string]models.Initiator{},
	}
	return ot
}

func (ot *OntTracker) Start() error {
	var merr error
	err := ot.store.Jobs(func(j *models.JobSpec) bool {
		merr = multierr.Append(merr, ot.AddJob(*j))
		return true
	}, models.InitiatorOntEvent)
	if merr != nil {
		return multierr.Append(merr, err)
	}
	ot.quit = make(chan struct{})
	go ot.track()
	return nil
}

func (ot *OntTracker) Stop() error {
	close(ot.quit)
	return nil
}

// AddJob subscribes to ontology log events for each "ontevent"
// initiator in the passed job spec.
func (ot *OntTracker) AddJob(job models.JobSpec) error {
	if !job.IsOntEventInitiated() {
		return nil
	}

	initrs := job.InitiatorsFor(
		models.InitiatorOntEvent,
	)

	ot.mutex.Lock()
	for _, initr := range initrs {
		address, err := ontcommon.AddressParseFromBytes(initr.Address[:])
		if err != nil {
			return err
		}
		initrMap, ok := ot.contractSubscriptions[address.ToHexString()]
		if !ok {
			initrMap = map[string]models.Initiator{}
		}
		initrMap[job.ID.String()] = initr
		ot.contractSubscriptions[address.ToHexString()] = initrMap
		logger.Debugw("ont tracker, add new job", "jobID", job.ID.String(), "address", address.ToHexString())
	}
	ot.jobSubscriptions[job.ID.String()] = job
	ot.mutex.Unlock()
	return nil
}

// RemoveJob unsubscribes the job from a log subscription
func (ot *OntTracker) RemoveJob(ID *models.ID) error {
	ot.mutex.Lock()
	job, ok := ot.jobSubscriptions[ID.String()]
	if !ok {
		return fmt.Errorf("ontTracker#RemoveJob: job %s not found", ID)
	}

	initrs := job.InitiatorsFor(
		models.InitiatorOntEvent,
	)
	for _, initr := range initrs {
		address, err := ontcommon.AddressParseFromBytes(initr.Address[:])
		if err != nil {
			return err
		}
		delete(ot.contractSubscriptions[address.ToHexString()], ID.String())
		if len(ot.contractSubscriptions[address.ToHexString()]) == 0 {
			delete(ot.contractSubscriptions, address.ToHexString())
		}
	}
	delete(ot.jobSubscriptions, ID.String())
	ot.mutex.Unlock()
	return nil
}

// Jobs returns the jobs being listened to.
func (ot *OntTracker) Jobs() []models.JobSpec {
	ot.mutex.RLock()
	defer ot.mutex.RUnlock()

	var jobs []models.JobSpec
	for _, job := range ot.jobSubscriptions {
		jobs = append(jobs, job)
	}
	return jobs
}

func (ot *OntTracker) track() {
	currentHeight, err := ot.ontTxManager.GetCurrentBlockHeight()
	if err != nil {
		logger.Error("ont tracker, get current block height error:", err)
	}
	ontHeight, err := ot.store.FirstOrCreateOntHeight(models.NewOntHeight(currentHeight))
	if err != nil {
		logger.Error("ont tracker, find or create ontology current track height error:", err)
	}
	ot.height = ontHeight.Height

	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			currentHeight, err := ot.ontTxManager.GetCurrentBlockHeight()
			if err != nil {
				logger.Error("ont tracker, get current block height error:", err)
				continue
			}
			for h := ot.height; h < currentHeight+1; h++ {
				err := ot.ParseOntEvent(h)
				if err != nil {
					logger.Error("ont tracker, parse ont event error:", err)
				}
			}
			ot.height = currentHeight + 1
			err = ot.store.SaveOntHeight(models.NewOntHeight(currentHeight + 1))
			if err != nil {
				logger.Error("ont tracker, save current block height to db error:", err)
			}
		case <-ot.quit:
			ticker.Stop()
			return
		}
	}
}

func (ot *OntTracker) ParseOntEvent(height uint32) error {
	events, err := ot.ontTxManager.GetSmartContractEventByBlock(height)
	logger.Debugw("ont tracker, start to parse ont block", "height", height)
	if err != nil {
		return fmt.Errorf("get smartcontract event by block error:%s", err)
	}

	for _, event := range events {
		for _, notify := range event.Notify {
			states, ok := notify.States.([]interface{})
			if !ok {
				continue
			}
			_, ok = ot.contractSubscriptions[notify.ContractAddress]
			if !ok {
				continue
			}
			name := states[0].(string)
			if name == hex.EncodeToString([]byte("oracleRequest")) {
				jobId := states[1].(string)
				jobSpec, ok := ot.jobSubscriptions[jobId]
				if !ok {
					continue
				}
				logger.Debugw("ont tracker, found tracked job", "jobId", jobId)
				initiator := ot.contractSubscriptions[notify.ContractAddress][jobId]
				txHash, err := ontcommon.Uint256FromHexString(event.TxHash)
				if err != nil {
					logger.Error("ont tracker, txhash from hex to ontology txhash error:", err)
				}
				rrTxHash := common.BytesToHash(txHash[:])
				sender := states[2].(string)
				senderBytes, err := hex.DecodeString(sender)
				if err != nil {
					logger.Error("ont tracker, address from hex to bytes error:", err)
				}
				address, err := ontcommon.AddressParseFromBytes(senderBytes)
				if err != nil {
					logger.Error("ont tracker, address from hex to ontology address error:", err)
				}
				rrSender := common.BytesToAddress(address[:])
				requestID := states[3].(string)
				p := states[4].(string)
				paymentBytes, err := hex.DecodeString(p)
				if err != nil {
					logger.Error("ont tracker, payment from hex to bytes error:", err)
				}
				payment := ontcommon.BigIntFromNeoBytes(paymentBytes)
				rrPayment := assets.NewLink(payment.Int64())
				callbackAddress := states[5].(string)
				function := states[6].(string)
				expiration := states[7].(string)
				data := states[9].(string)
				dataBytes, err := hex.DecodeString(data)
				if err != nil {
					logger.Error("ont tracker, date from hex to bytes error:", err)
				}
				js, err := models.ParseCBOR(dataBytes)
				if err != nil {
					logger.Error("ont tracker, date from bytes to JSON error:", err)
				}
				js, err = js.Add("address", notify.ContractAddress)
				if err != nil {
					logger.Error("ont tracker, date JSON add address error:", err)
				}
				js, err = js.Add("requestID", requestID)
				if err != nil {
					logger.Error("ont tracker, date JSON add requestID error:", err)
				}
				js, err = js.Add("payment", p)
				if err != nil {
					logger.Error("ont tracker, date JSON add payment error:", err)
				}
				js, err = js.Add("callbackAddress", callbackAddress)
				if err != nil {
					logger.Error("ont tracker, date JSON add callbackAddress error:", err)
				}
				js, err = js.Add("callbackFunction", function)
				if err != nil {
					logger.Error("ont tracker, date JSON add callbackFunction error:", err)
				}
				js, err = js.Add("expiration", expiration)
				if err != nil {
					logger.Error("ont tracker, date JSON add expiration error:", err)
				}
				rr := models.RunRequest{
					RequestID:     &requestID,
					TxHash:        &rrTxHash,
					Requester:     &rrSender,
					Payment:       rrPayment,
					RequestParams: js,
				}
				_, err = ot.runManager.Create(jobSpec.ID, &initiator, nil, &rr)
				if err != nil {
					logger.Error("ont tracker, runManager create job run error:", err)
				}
			}
		}
	}
	return nil
}
