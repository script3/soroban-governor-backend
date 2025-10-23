package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/script3/soroban-governor-backend/internal/governor"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

//********** History Table **********//

const (
	HISTORY_TABLE_NAME = "history"
	HISTORY_COLUMNS    = "event_id, contract_id, proposal_id, event_type, event_data, tx_hash, ledger_seq, ledger_close_time"
)

func historyArgs(event *governor.GovernorEvent) []any {
	return []any{
		event.EventId,
		event.ContractId,
		event.ProposalId,
		event.EventType,
		event.EventData,
		event.TxHash,
		event.LedgerSeq,
		event.LedgerCloseTime,
	}
}

func scanHistoryEvent(scanner interface{ Scan(...any) error }) (*governor.GovernorEvent, error) {
	event := &governor.GovernorEvent{}
	err := scanner.Scan(
		&event.EventId,
		&event.ContractId,
		&event.ProposalId,
		&event.EventType,
		&event.EventData,
		&event.TxHash,
		&event.LedgerSeq,
		&event.LedgerCloseTime,
	)
	return event, err
}

// InsertEvent inserts a new governor event into the history table
func (store *Store) InsertEvent(ctx context.Context, event *governor.GovernorEvent) error {
	query := fmt.Sprintf(`
        INSERT INTO %s (%s) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
        ON CONFLICT (event_id) DO NOTHING`,
		HISTORY_TABLE_NAME, HISTORY_COLUMNS,
	)

	_, err := store.db.ExecContext(
		ctx,
		query,
		historyArgs(event)...,
	)

	return err
}

// GetEventById retrieves a single event by its ID
func (store *Store) GetEvent(ctx context.Context, eventId string) (*governor.GovernorEvent, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE event_id = $1
	`, HISTORY_COLUMNS, HISTORY_TABLE_NAME)

	event, err := scanHistoryEvent(store.db.QueryRowContext(ctx, query, eventId))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return event, nil
}

// GetEventsByContractId retrieves events with pagination
// TODO: add pagination
func (store *Store) GetEventsByContractId(
	ctx context.Context,
	contractId string,
) ([]*governor.GovernorEvent, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE contract_id = $1
		ORDER BY event_id ASC
	`, HISTORY_COLUMNS, HISTORY_TABLE_NAME)

	rows, err := store.db.QueryContext(ctx, query, contractId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*governor.GovernorEvent
	for rows.Next() {
		event, err := scanHistoryEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return events, nil
}

//********** Status Table Methods **********//

// UpsertLedgerSeq updates the last stored ledger sequence in the status table
func (store *Store) UpsertLedgerSeq(ctx context.Context, source string, ledgerSeq uint32) error {
	query := `
		INSERT INTO status (source, ledger_seq)
		VALUES ($1, $2)
		ON CONFLICT (source) DO UPDATE SET ledger_seq = EXCLUDED.ledger_seq
	`
	_, err := store.db.ExecContext(ctx, query, source, ledgerSeq)
	return err
}

// GetLedgerSeq returns the last stored ledger sequence in the status table
func (store *Store) GetLedgerSeq(ctx context.Context, source string) (uint32, error) {
	query := `SELECT ledger_seq FROM status WHERE source = $1`

	var ledgerSeq uint32
	err := store.db.QueryRowContext(ctx, query, source).Scan(&ledgerSeq)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}

	return ledgerSeq, nil
}

//********** Proposals Table **********//

const (
	PROPOSALS_TABLE_NAME = "proposals"
	PROPOSALS_COLUMNS    = "proposal_key, contract_id, proposal_id, proposer, status, title, description, action, vote_start, vote_end, votes_for, votes_against, votes_abstain, execution_unlock, execution_tx_hash"
)

func proposalArgs(proposal *governor.Proposal) []any {
	return []any{
		proposal.ProposalKey,
		proposal.ContractId,
		proposal.ProposalId,
		proposal.Proposer,
		proposal.Status,
		proposal.Title,
		proposal.Description,
		proposal.Action,
		proposal.VoteStart,
		proposal.VoteEnd,
		proposal.VotesFor,
		proposal.VotesAgainst,
		proposal.VotesAbstain,
		proposal.ExecutionUnlock,
		proposal.ExecutionTxHash,
	}
}

func scanProposal(scanner interface{ Scan(...any) error }) (*governor.Proposal, error) {
	proposal := &governor.Proposal{}
	err := scanner.Scan(
		&proposal.ProposalKey,
		&proposal.ContractId,
		&proposal.ProposalId,
		&proposal.Proposer,
		&proposal.Status,
		&proposal.Title,
		&proposal.Description,
		&proposal.Action,
		&proposal.VoteStart,
		&proposal.VoteEnd,
		&proposal.VotesFor,
		&proposal.VotesAgainst,
		&proposal.VotesAbstain,
		&proposal.ExecutionUnlock,
		&proposal.ExecutionTxHash,
	)
	return proposal, err
}

// UpsertProposal inserts or updates a proposal in the proposals table
// For updates, it ignores fixed fields, and only updates mutable fields (votes_*, execution_*, status)
func (store *Store) UpsertProposal(ctx context.Context, proposal *governor.Proposal) error {
	// @dev note: doesn't update proposal_key, contract_id, proposal_id on conflict
	// to prevent changing primary identifiers
	query := fmt.Sprintf(`
		INSERT INTO %s (%s) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		ON CONFLICT (proposal_key) 
		DO UPDATE SET 
			status = EXCLUDED.status,
			votes_for = EXCLUDED.votes_for,
			votes_against = EXCLUDED.votes_against,
			votes_abstain = EXCLUDED.votes_abstain,
			execution_unlock = EXCLUDED.execution_unlock,
			execution_tx_hash = EXCLUDED.execution_tx_hash
		`, PROPOSALS_TABLE_NAME, PROPOSALS_COLUMNS)

	_, err := store.db.ExecContext(
		ctx,
		query,
		proposalArgs(proposal)...,
	)

	return err
}

// GetProposal retrieves a proposal by its unique proposal key
func (store *Store) GetProposal(ctx context.Context, proposalKey string) (*governor.Proposal, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE proposal_key = $1
	`, PROPOSALS_COLUMNS, PROPOSALS_TABLE_NAME)

	proposal, err := scanProposal(store.db.QueryRowContext(ctx, query, proposalKey))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return proposal, nil
}

// GetProposalsByContract retrieves all proposals for a given contract ID
// TODO: add pagination
func (store *Store) GetProposalsByContractId(ctx context.Context, contractId string) ([]*governor.Proposal, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE contract_id = $1
		ORDER BY proposal_id DESC
	`, PROPOSALS_COLUMNS, PROPOSALS_TABLE_NAME)

	rows, err := store.db.QueryContext(ctx, query, contractId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proposals []*governor.Proposal
	for rows.Next() {
		proposal, err := scanProposal(rows)
		if err != nil {
			return nil, err
		}
		proposals = append(proposals, proposal)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return proposals, nil
}

//********** Votes Table **********//

const (
	VOTES_TABLE_NAME = "votes"
	VOTES_COLUMNS    = "tx_hash, contract_id, proposal_id, voter, support, amount, ledger_seq, ledger_close_time"
)

func voteArgs(vote *governor.Vote) []any {
	return []any{
		vote.TxHash,
		vote.ContractId,
		vote.ProposalId,
		vote.Voter,
		vote.Support,
		vote.Amount,
		vote.LedgerSeq,
		vote.LedgerCloseTime,
	}
}

func scanVote(scanner interface{ Scan(...any) error }) (*governor.Vote, error) {
	vote := &governor.Vote{}
	err := scanner.Scan(
		&vote.TxHash,
		&vote.ContractId,
		&vote.ProposalId,
		&vote.Voter,
		&vote.Support,
		&vote.Amount,
		&vote.LedgerSeq,
		&vote.LedgerCloseTime,
	)
	return vote, err
}

func (store *Store) InsertVote(ctx context.Context, vote *governor.Vote) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (%s) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (tx_hash) DO NOTHING
		`, VOTES_TABLE_NAME, VOTES_COLUMNS)

	_, err := store.db.ExecContext(
		ctx,
		query,
		voteArgs(vote)...,
	)

	return err
}

func (store *Store) GetVote(ctx context.Context, txHash string) (*governor.Vote, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE tx_hash = $1
	`, VOTES_COLUMNS, VOTES_TABLE_NAME)

	vote, err := scanVote(store.db.QueryRowContext(ctx, query, txHash))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return vote, nil
}

func (store *Store) GetVotesByProposal(ctx context.Context, contractId string, proposalId uint32) ([]*governor.Vote, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM %s
		WHERE contract_id = $1 AND proposal_id = $2
		ORDER BY ledger_seq DESC
	`, VOTES_COLUMNS, VOTES_TABLE_NAME)

	rows, err := store.db.QueryContext(ctx, query, contractId, proposalId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []*governor.Vote
	for rows.Next() {
		vote, err := scanVote(rows)
		if err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return votes, nil
}
