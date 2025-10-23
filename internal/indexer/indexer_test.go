package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/google/go-cmp/cmp"
	"github.com/script3/soroban-governor-backend/internal/db"
	"github.com/script3/soroban-governor-backend/internal/governor"
)

// the DB's initial state. Placed at global scope so it can be reused across tests.
// Note - this doesn't necessarily make sense, but provides enough data to test against.
// since arrays are mutable this is not a const, so plz don't modify it.
var (
	ledgerSeq       = uint32(1170234)
	ledgerCloseTime = int64(1761053041)
	testContractId  = "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB"
	initHistory     = []*governor.GovernorEvent{
		{
			EventId:         "0005025695851876451-0000000042",
			ContractId:      testContractId,
			EventType:       "vote_cast",
			ProposalId:      2,
			EventData:       `{"voter":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","support":0,"amount":"20000000000"}`,
			TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
			LedgerSeq:       ledgerSeq - 100,
			LedgerCloseTime: ledgerCloseTime - 500,
		},
	}
	initProposals = []*governor.Proposal{
		{ // active proposal
			ProposalKey:     fmt.Sprintf("%s-3", testContractId),
			ContractId:      testContractId,
			ProposalId:      3,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          0,
			Title:           "Unicorns are real",
			Description:     "They live in the clouds",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       ledgerSeq - 10000,
			VoteEnd:         ledgerSeq,
			VotesFor:        "12314122341234",
			VotesAgainst:    "1234123412434",
			VotesAbstain:    "1923114243",
			ExecutionUnlock: 0,
			ExecutionTxHash: "",
		},
		{ // defeated proposal
			ProposalKey:     fmt.Sprintf("%s-2", testContractId),
			ContractId:      testContractId,
			ProposalId:      2,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          2,
			Title:           "Unicorns are fake",
			Description:     "They don't live anywhere",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       ledgerSeq - 30000,
			VoteEnd:         ledgerSeq - 20000,
			VotesFor:        "123141223412",
			VotesAgainst:    "984723948572235",
			VotesAbstain:    "594114243",
			ExecutionUnlock: 0,
			ExecutionTxHash: "",
		},
		{ // successful proposal
			ProposalKey:     fmt.Sprintf("%s-1", testContractId),
			ContractId:      testContractId,
			ProposalId:      1,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          1,
			Title:           "Unicorns need more research",
			Description:     "They could exist somewhere",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       ledgerSeq - 40000,
			VoteEnd:         ledgerSeq - 30000,
			VotesFor:        "123141223412",
			VotesAgainst:    "984723948572235",
			VotesAbstain:    "594114243",
			ExecutionUnlock: ledgerSeq - 1000,
			ExecutionTxHash: "",
		},
		{ // executed proposal
			ProposalKey:     fmt.Sprintf("%s-0", testContractId),
			ContractId:      testContractId,
			ProposalId:      0,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          4,
			Title:           "Unicorns are magical",
			Description:     "They sparkle",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       ledgerSeq - 50000,
			VoteEnd:         ledgerSeq - 40000,
			VotesFor:        "123141223412",
			VotesAgainst:    "984723948572235",
			VotesAbstain:    "594114243",
			ExecutionUnlock: ledgerSeq - 10000,
			ExecutionTxHash: "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
		},
	}
	initVotes = []*governor.Vote{
		{
			TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
			ContractId:      testContractId,
			ProposalId:      3,
			Voter:           "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Support:         0,
			Amount:          "123450000000",
			LedgerSeq:       ledgerSeq - 1234,
			LedgerCloseTime: ledgerCloseTime - (1234 * 5),
		},
	}
)

// setupStore creates an in-memory SQLite database for testing
// also initializes the in-memory DB with the test data
func setupStore(t *testing.T, ctx context.Context) *db.Store {
	t.Helper()

	// Create in-memory database
	sqlDb, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := db.RunMigrations(sqlDb); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Cleanup function to close database after test
	t.Cleanup(func() {
		sqlDb.Close()
	})

	store := db.NewStore(sqlDb)

	// Initialize with test data
	for _, event := range initHistory {
		err := store.InsertEvent(ctx, event)
		if err != nil {
			t.Fatalf("failed to insert initial governor event: %v", err)
		}
	}

	for _, proposal := range initProposals {
		err := store.UpsertProposal(ctx, proposal)
		if err != nil {
			t.Fatalf("failed to insert initial proposal: %v", err)
		}
	}

	for _, vote := range initVotes {
		err := store.InsertVote(ctx, vote)
		if err != nil {
			t.Fatalf("failed to insert initial vote: %v", err)
		}
	}

	return store
}

func TestApplyEvent(t *testing.T) {
	tests := []struct {
		name         string
		event        *governor.GovernorEvent
		wantProposal *governor.Proposal
		wantVote     *governor.Vote
		wantErr      bool
	}{
		{
			name: "proposal_created happy path",
			event: &governor.GovernorEvent{
				EventId:    "0005025687261941760-0000000000",
				ContractId: testContractId,
				EventType:  "proposal_created",
				ProposalId: 4,
				EventData: fmt.Sprintf(
					`{"proposer":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","title":"Make me security council","desc":"plz","action":"AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl","vote_start":%d,"vote_end":%d}`,
					ledgerSeq+1000,
					ledgerSeq+21000,
				),
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: &governor.Proposal{
				ProposalKey:     fmt.Sprintf("%s-4", testContractId),
				ContractId:      testContractId,
				ProposalId:      4,
				Proposer:        "GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q",
				Status:          0,
				Title:           "Make me security council",
				Description:     "plz",
				Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
				VoteStart:       ledgerSeq + 1000,
				VoteEnd:         ledgerSeq + 21000,
				VotesFor:        "0",
				VotesAgainst:    "0",
				VotesAbstain:    "0",
				ExecutionUnlock: 0,
				ExecutionTxHash: "",
			},
			wantVote: nil,
			wantErr:  false,
		},
		{
			name: "proposal_created duplicate does nothing",
			event: &governor.GovernorEvent{
				EventId:    "0005025687261941760-0000000000",
				ContractId: testContractId,
				EventType:  "proposal_created",
				ProposalId: 3,
				EventData: fmt.Sprintf(
					`{"proposer":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","title":"Make me security council","desc":"plz","action":"AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl","vote_start":%d,"vote_end":%d}`,
					ledgerSeq+1000,
					ledgerSeq+21000,
				),
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: initProposals[0],
			wantVote:     nil,
			wantErr:      true,
		},
		{
			name: "proposal_canceled happy path",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_canceled",
				ProposalId:      3,
				EventData:       "{}",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: &governor.Proposal{
				ProposalKey:     fmt.Sprintf("%s-3", testContractId),
				ContractId:      testContractId,
				ProposalId:      3,
				Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
				Status:          5,
				Title:           "Unicorns are real",
				Description:     "They live in the clouds",
				Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
				VoteStart:       ledgerSeq - 10000,
				VoteEnd:         ledgerSeq,
				VotesFor:        "12314122341234",
				VotesAgainst:    "1234123412434",
				VotesAbstain:    "1923114243",
				ExecutionUnlock: 0,
				ExecutionTxHash: "",
			},
			wantVote: nil,
			wantErr:  false,
		},
		{
			name: "proposal_canceled no proposal fails",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_canceled",
				ProposalId:      4,
				EventData:       "{}",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: nil,
			wantVote:     nil,
			wantErr:      true,
		},
		{
			name: "proposal_canceled invalid status does not apply",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_canceled",
				ProposalId:      2,
				EventData:       "{}",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: initProposals[1],
			wantVote:     nil,
			wantErr:      false,
		},
		{
			name: "proposal_voting_closed happy path",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_voting_closed",
				ProposalId:      3,
				EventData:       `{"status":1,"eta":1120234,"final_votes":{"for":"50230000000","against":"20000000000","abstain":"123"}}`,
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: &governor.Proposal{
				ProposalKey:     fmt.Sprintf("%s-3", testContractId),
				ContractId:      testContractId,
				ProposalId:      3,
				Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
				Status:          1,
				Title:           "Unicorns are real",
				Description:     "They live in the clouds",
				Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
				VoteStart:       ledgerSeq - 10000,
				VoteEnd:         ledgerSeq,
				VotesFor:        "50230000000",
				VotesAgainst:    "20000000000",
				VotesAbstain:    "123",
				ExecutionUnlock: 1120234,
				ExecutionTxHash: "",
			},
			wantVote: nil,
			wantErr:  false,
		},
		{
			name: "proposal_voting_closed no proposal fails",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_voting_closed",
				ProposalId:      4,
				EventData:       `{"status":1,"eta":1120234,"final_votes":{"for":"50230000000","against":"20000000000","abstain":"123"}}`,
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: nil,
			wantVote:     nil,
			wantErr:      true,
		},
		{
			name: "proposal_voting_closed invalid status does not apply",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_voting_closed",
				ProposalId:      2,
				EventData:       `{"status":1,"eta":1120234,"final_votes":{"for":"50230000000","against":"20000000000","abstain":"123"}}`,
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: initProposals[1],
			wantVote:     nil,
			wantErr:      false,
		},
		{
			name: "proposal_executed happy path",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_executed",
				ProposalId:      1,
				EventData:       "",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: &governor.Proposal{
				ProposalKey:     fmt.Sprintf("%s-1", testContractId),
				ContractId:      testContractId,
				ProposalId:      1,
				Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
				Status:          4,
				Title:           "Unicorns need more research",
				Description:     "They could exist somewhere",
				Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
				VoteStart:       ledgerSeq - 40000,
				VoteEnd:         ledgerSeq - 30000,
				VotesFor:        "123141223412",
				VotesAgainst:    "984723948572235",
				VotesAbstain:    "594114243",
				ExecutionUnlock: ledgerSeq - 1000,
				ExecutionTxHash: "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
			},
			wantVote: nil,
			wantErr:  false,
		},
		{
			name: "proposal_executed no proposal fails",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_executed",
				ProposalId:      4,
				EventData:       "",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: nil,
			wantVote:     nil,
			wantErr:      true,
		},
		{
			name: "proposal_executed invalid status does not apply",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_executed",
				ProposalId:      0,
				EventData:       "",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: initProposals[3],
			wantVote:     nil,
			wantErr:      false,
		},
		{
			name: "proposal_expired happy path",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_expired",
				ProposalId:      3,
				EventData:       "{}",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: &governor.Proposal{
				ProposalKey:     fmt.Sprintf("%s-3", testContractId),
				ContractId:      testContractId,
				ProposalId:      3,
				Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
				Status:          3,
				Title:           "Unicorns are real",
				Description:     "They live in the clouds",
				Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
				VoteStart:       ledgerSeq - 10000,
				VoteEnd:         ledgerSeq,
				VotesFor:        "12314122341234",
				VotesAgainst:    "1234123412434",
				VotesAbstain:    "1923114243",
				ExecutionUnlock: 0,
				ExecutionTxHash: "",
			},
			wantVote: nil,
			wantErr:  false,
		},
		{
			name: "proposal_expired no proposal fails",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_expired",
				ProposalId:      4,
				EventData:       "{}",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: nil,
			wantVote:     nil,
			wantErr:      true,
		},
		{
			name: "proposal_expired invalid status does not apply",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "proposal_expired",
				ProposalId:      2,
				EventData:       "{}",
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: initProposals[1],
			wantVote:     nil,
			wantErr:      false,
		},
		{
			name: "vote_cast happy path",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "vote_cast",
				ProposalId:      3,
				EventData:       `{"voter":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","support":1,"amount":"20000000000"}`,
				TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: &governor.Proposal{
				ProposalKey:     fmt.Sprintf("%s-3", testContractId),
				ContractId:      testContractId,
				ProposalId:      3,
				Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
				Status:          0,
				Title:           "Unicorns are real",
				Description:     "They live in the clouds",
				Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
				VoteStart:       ledgerSeq - 10000,
				VoteEnd:         ledgerSeq,
				VotesFor:        "12334122341234",
				VotesAgainst:    "1234123412434",
				VotesAbstain:    "1923114243",
				ExecutionUnlock: 0,
				ExecutionTxHash: "",
			},
			wantVote: &governor.Vote{
				TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
				ContractId:      testContractId,
				ProposalId:      3,
				Voter:           "GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q",
				Support:         1,
				Amount:          "20000000000",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantErr: false,
		},
		{
			name: "vote_cast no proposal fails",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "vote_cast",
				ProposalId:      4,
				EventData:       `{"voter":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","support":1,"amount":"20000000000"}`,
				TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: nil,
			wantVote:     nil,
			wantErr:      true,
		},
		{
			name: "vote_cast invalid status does not apply",
			event: &governor.GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      testContractId,
				EventType:       "vote_cast",
				ProposalId:      2,
				EventData:       `{"voter":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","support":1,"amount":"20000000000"}`,
				TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
				LedgerSeq:       ledgerSeq,
				LedgerCloseTime: ledgerCloseTime,
			},
			wantProposal: initProposals[1],
			wantVote:     nil,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			store := setupStore(t, ctx)

			indexer := NewIndexer(store)

			err := indexer.ApplyEvent(ctx, tt.event)
			if err != nil && !tt.wantErr {
				t.Fatalf("ApplyEvent() error = %v", err)
			} else if err == nil && tt.wantErr {
				t.Fatalf("ApplyEvent() expected error but got none")
			}

			event, err := store.GetEvent(ctx, tt.event.EventId)
			if err != nil {
				t.Fatalf("failed to get event from history: %v", err)
			}
			if diff := cmp.Diff(tt.event, event); diff != "" {
				t.Errorf("event mismatch (-want +got):\n%s", diff)
			}

			if tt.wantProposal != nil {
				proposalKey := governor.EncodeProposalKey(tt.event.ContractId, tt.event.ProposalId)
				proposal, err := store.GetProposal(ctx, proposalKey)
				if err != nil {
					t.Fatalf("failed to get proposal after ApplyEvent: %v", err)
				}
				if diff := cmp.Diff(tt.wantProposal, proposal); diff != "" {
					t.Errorf("proposal mismatch (-want +got):\n%s", diff)
				}
			}

			if tt.wantVote != nil {
				vote, err := store.GetVote(ctx, tt.event.TxHash)
				if err != nil {
					t.Fatalf("failed to get vote after ApplyEvent: %v", err)
				}
				if diff := cmp.Diff(tt.wantVote, vote); diff != "" {
					t.Errorf("vote mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
