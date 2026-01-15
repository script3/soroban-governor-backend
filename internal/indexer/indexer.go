package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"

	"github.com/script3/soroban-governor-backend/internal/db"
	"github.com/script3/soroban-governor-backend/internal/governor"
	"github.com/stellar/go-stellar-sdk/ingest"
	"github.com/stellar/go-stellar-sdk/toid"
	"github.com/stellar/go-stellar-sdk/xdr"
)

type Indexer struct {
	store *db.Store
}

func NewIndexer(store *db.Store) *Indexer {
	return &Indexer{store: store}
}

// ApplyLedger processes all transactions in a ledger and applies relevant governor events to the db
func (idx *Indexer) ApplyLedger(ctx context.Context, txReader *ingest.LedgerTransactionReader, ledgerSeq uint32, ledgerCloseTime int64) (int, error) {
	txCount := 0
	for {
		tx, err := txReader.Read()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return txCount, fmt.Errorf("failed to read ledger transaction: %w", err)
			}
		}
		txCount++

		if !tx.Successful() {
			continue
		}

		// currently, only process events from InvokeHostFunction operations, which must be the one and only operation
		op_0, ok := tx.GetOperation(0)
		if !ok {
			continue
		}
		if op_0.Body.Type != xdr.OperationTypeInvokeHostFunction {
			continue
		}

		events, err := tx.GetContractEvents()
		if err != nil {
			slog.Error("Failed getting events for tx", "ledger", ledgerSeq, "hash", tx.Hash, "err", err)
			continue
		}

		toidInt := toid.New(int32(ledgerSeq), int32(tx.Index), 0).ToInt64()

		for event_index, event := range events {
			govEvent, err := governor.NewGovernorEventFromContractEvent(&event, tx.Hash.HexString(), ledgerSeq, int64(ledgerCloseTime), toidInt, int32(event_index))
			if err != nil {
				// only log failures for events if we think it is a governor event
				if errors.Is(err, governor.ErrEventParsingFailed) {
					eventStr, xdrErr := xdr.MarshalBase64(event)
					if xdrErr != nil {
						slog.Error("Failed parsing and unable to marshal xdr", "ledger", ledgerSeq, "hash", tx.Hash.HexString(), "xdrErr", xdrErr)
					} else {
						slog.Error("Failed parsing event", "ledger", ledgerSeq, "hash", tx.Hash.HexString(), "event", eventStr, "err", err)
					}
				}
				continue
			}

			applyErr := idx.ApplyEvent(ctx, govEvent)
			if applyErr != nil {
				slog.Error("Failed applying event to db", "ledger", ledgerSeq, "hash", tx.Hash.HexString(), "event", govEvent, "err", applyErr)
				continue
			}
		}
	}
	return txCount, nil
}

// ApplyEvent processes a GovernorEvent and applies changes to aggregated tables
//
// It is assumed that the event already exists in the event history table
func (idx *Indexer) ApplyEvent(ctx context.Context, govEvent *governor.GovernorEvent) error {
	slog.Info("Applying event", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "eventId", govEvent.EventId)
	// store the event into the event history
	// this (eventually) should be functional to replay / rehydrate the aggregated db services
	// its also dupe safe, so running this for an event that already exists is a no-op
	err := idx.store.InsertEvent(ctx, govEvent)
	if err != nil {
		return fmt.Errorf("failed to insert event into history: %w", err)
	}

	// check if the proposal exists
	proposal, err := idx.store.GetProposal(ctx, governor.EncodeProposalKey(govEvent.ContractId, govEvent.ProposalId))
	if err != nil {
		return fmt.Errorf("error when attempting to get proposal from store: %w", err)
	}

	switch govEvent.EventType {
	case "proposal_created":
		if proposal == nil {
			proposal, err = governor.NewProposalFromProposalCreatedEvent(govEvent)
			if err != nil {
				return fmt.Errorf("failed to create proposal from event: %w", err)
			}
		} else {
			return fmt.Errorf("proposal_created event for existing proposal %v status: %d", proposal.ProposalKey, proposal.Status)
		}
	case "proposal_canceled":
		if proposal == nil {
			return fmt.Errorf("proposal_canceled event for non-existing proposal %s-%d", govEvent.ContractId, govEvent.ProposalId)
		} else if proposal.Status != 0 {
			slog.Info("proposal_canceled event for proposal not in active state", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "proposal", proposal.ProposalKey, "current_status", proposal.Status)
			return nil
		}
		proposal.Status = 5
	case "proposal_voting_closed":
		if proposal == nil {
			return fmt.Errorf("proposal_voting_closed event for non-existing proposal %s-%d", govEvent.ContractId, govEvent.ProposalId)
		} else if proposal.Status != 0 {
			slog.Info("proposal_voting_closed event for proposal not in active state", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "proposal", proposal.ProposalKey, "current_status", proposal.Status)
			return nil
		}
		var votingClosedData *governor.ProposalVotingClosedData
		err = json.Unmarshal([]byte(govEvent.EventData), &votingClosedData)
		if err != nil {
			return fmt.Errorf("unable to unmarshal proposal_voting_closed event data: %w", err)
		}
		proposal.Status = votingClosedData.Status
		proposal.VotesFor = votingClosedData.FinalVotes.For
		proposal.VotesAgainst = votingClosedData.FinalVotes.Against
		proposal.VotesAbstain = votingClosedData.FinalVotes.Abstain
		proposal.ExecutionUnlock = votingClosedData.Eta
	case "proposal_executed":
		if proposal == nil {
			return fmt.Errorf("proposal_executed event for non-existing proposal %s-%d", govEvent.ContractId, govEvent.ProposalId)
		} else if proposal.Status == 4 {
			slog.Info("proposal_executed event for proposal that has already been executed", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "proposal", proposal.ProposalKey, "execution_tx_hash", proposal.ExecutionTxHash)
			return nil
		}
		proposal.Status = 4
		proposal.ExecutionTxHash = govEvent.TxHash
	case "proposal_expired":
		if proposal == nil {
			return fmt.Errorf("proposal_expired event for non-existing proposal %s-%d", govEvent.ContractId, govEvent.ProposalId)
		} else if proposal.Status != 0 && proposal.Status != 1 {
			slog.Info("proposal_expired event for proposal not in active state", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "proposal", proposal.ProposalKey, "current_status", proposal.Status)
			return nil
		}
		proposal.Status = 3
	case "vote_cast":
		if proposal == nil {
			return fmt.Errorf("vote_cast event for non-existing proposal %s-%d", govEvent.ContractId, govEvent.ProposalId)
		} else if proposal.Status != 0 {
			slog.Info("vote_cast event for proposal not in active state", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "proposal", proposal.ProposalKey, "current_status", proposal.Status)
			return nil
		}
		var voteCastData *governor.VoteCastData
		err = json.Unmarshal([]byte(govEvent.EventData), &voteCastData)
		if err != nil {
			return fmt.Errorf("unable to unmarshal vote_cast event data: %w", err)
		}

		curVote, err := idx.store.GetVote(ctx, govEvent.TxHash)
		if err != nil {
			return fmt.Errorf("error when attempting to get vote from store: %w", err)
		}
		if curVote != nil {
			slog.Info("vote_cast event already applied", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "proposal", proposal.ProposalKey, "current_status", proposal.Status)
			return nil
		}

		amountBig, ok := new(big.Int).SetString(voteCastData.Amount, 10)
		if !ok {
			return fmt.Errorf("invalid amount string %s in vote_cast event", voteCastData.Amount)
		}

		switch voteCastData.Support {
		case 0:
			// against
			totalAgainst, ok := new(big.Int).SetString(proposal.VotesAgainst, 10)
			if !ok {
				return fmt.Errorf("invalid votes_against string %s in proposal %s", proposal.VotesAgainst, proposal.ProposalKey)
			}
			totalAgainst.Add(totalAgainst, amountBig)
			proposal.VotesAgainst = totalAgainst.String()

		case 1:
			// for
			totalFor, ok := new(big.Int).SetString(proposal.VotesFor, 10)
			if !ok {
				return fmt.Errorf("invalid votes_for string %s in proposal %s", proposal.VotesFor, proposal.ProposalKey)
			}
			totalFor.Add(totalFor, amountBig)
			proposal.VotesFor = totalFor.String()
		case 2:
			// abstain
			totalAbstain, ok := new(big.Int).SetString(proposal.VotesAbstain, 10)
			if !ok {
				return fmt.Errorf("invalid votes_abstain string %s in proposal %s", proposal.VotesAbstain, proposal.ProposalKey)
			}
			totalAbstain.Add(totalAbstain, amountBig)
			proposal.VotesAbstain = totalAbstain.String()
		default:
			return fmt.Errorf("invalid support value %d in vote_cast event", voteCastData.Support)
		}

		vote, err := governor.NewVoteFromVoteCastEvent(govEvent)
		if err != nil {
			return fmt.Errorf("failed to create vote from event: %w", err)
		}
		err = idx.store.InsertVote(ctx, vote)
		if err != nil {
			return fmt.Errorf("failed to insert vote into store: %w", err)
		}
	default:
		return fmt.Errorf("invalid event type %s", govEvent.EventType)
	}
	err = idx.store.UpsertProposal(ctx, proposal)
	if err != nil {
		return fmt.Errorf("failed to insert new proposal into store: %w", err)
	}
	slog.Info("Event applied successfully", "ledger", govEvent.LedgerSeq, "hash", govEvent.TxHash, "eventId", govEvent.EventId)
	return nil
}
