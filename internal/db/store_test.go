package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/script3/soroban-governor-backend/internal/governor"
	_ "modernc.org/sqlite"
)

// setupStore creates an in-memory SQLite database for testing
func setupStore(t *testing.T) *Store {
	t.Helper()

	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Run migrations
	if err := RunMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	// Cleanup function to close database after test
	t.Cleanup(func() {
		db.Close()
	})

	return NewStore(db)
}

func TestHistoryTable(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	events := []*governor.GovernorEvent{
		{
			EventId:         "0005025687261941760-0000000000",
			ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
			EventType:       "proposal_created",
			ProposalId:      3,
			EventData:       `{"proposer":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","title":"Make me security council","desc":"plz","action":"AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl","vote_start":1159020,"vote_end":1176300}`,
			TxHash:          "cb759f7b061992ac79e5f944a08238a24d2999a5ac58eee9fde35dff6404d970",
			LedgerSeq:       1170134,
			LedgerCloseTime: 1761053041,
		},
		{
			EventId:         "0005025695851872256-0000000001",
			ContractId:      "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
			EventType:       "proposal_canceled",
			ProposalId:      2,
			EventData:       `{}`,
			TxHash:          "cb759f7b061992ac79e5f944a08238a24d2999a5ac58eee9fde35dff6404d970",
			LedgerSeq:       1170136,
			LedgerCloseTime: 1761053046,
		},
		{
			EventId:         "0005025695851872256-0000000000",
			ContractId:      "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
			EventType:       "vote_cast",
			ProposalId:      2,
			EventData:       `{"voter":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","support":0,"amount":"20000000000"}`,
			TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
			LedgerSeq:       1170136,
			LedgerCloseTime: 1761053046,
		},
		{
			EventId:         "0005025700146839602-0000000003",
			ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
			EventType:       "proposal_voting_closed",
			ProposalId:      1,
			EventData:       `{"status":2,"eta":0,"final_votes":{"for":"1230000000","against":"20000000000","abstain":"0"}}`,
			TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
			LedgerSeq:       1170137,
			LedgerCloseTime: 1761053050,
		},
	}

	duplicateEvent := &governor.GovernorEvent{
		EventId:         events[0].EventId,
		ContractId:      "bad",
		EventType:       "bad",
		ProposalId:      99,
		EventData:       `bad`,
		TxHash:          "bad",
		LedgerSeq:       0,
		LedgerCloseTime: 0,
	}

	// insert all events
	for _, event := range events {
		err := store.InsertEvent(ctx, event)
		if err != nil {
			t.Fatalf("failed to insert event: %v", err)
		}
	}

	// test get event
	retrieved, err := store.GetEvent(ctx, events[0].EventId)
	if err != nil {
		t.Fatalf("failed to get event: %v", err)
	}
	if diff := cmp.Diff(events[0], retrieved); diff != "" {
		t.Errorf("check 1: mismatch (-want +got):\n%s", diff)
	}

	// test duplicate insert does nothing
	err = store.InsertEvent(ctx, duplicateEvent)
	if err != nil {
		t.Fatalf("failed to insert duplicate event: %v", err)
	}
	retrieved, err = store.GetEvent(ctx, events[0].EventId)
	if err != nil {
		t.Fatalf("failed to get event after duplicate insert: %v", err)
	}
	if diff := cmp.Diff(events[0], retrieved); diff != "" {
		t.Errorf("check 2: mismatch (-want +got):\n%s", diff)
	}

	// test get events by contract id
	retrievedEvents, err := store.GetEventsByContractId(ctx, events[1].ContractId)
	if err != nil {
		t.Fatalf("failed to get events by contract id: %v", err)
	}
	if len(retrievedEvents) != 2 {
		t.Fatalf("expected 2 events, got %d", len(retrievedEvents))
	}
	if diff := cmp.Diff(events[2], retrievedEvents[0]); diff != "" {
		t.Errorf("check 3a: mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(events[1], retrievedEvents[1]); diff != "" {
		t.Errorf("check 3b: mismatch (-want +got):\n%s", diff)
	}
}

func TestStatusTable(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	source := "indexer"

	// Set initial value
	err := store.UpsertLedgerSeq(ctx, source, 1000)
	if err != nil {
		t.Fatalf("failed to set initial ledger seq: %v", err)
	}

	// Verify value
	retrieved, err := store.GetLedgerSeq(ctx, source)
	if err != nil {
		t.Fatalf("failed to get ledger seq: %v", err)
	}

	if retrieved != 1000 {
		t.Errorf("expected ledger_seq 2000, got %d", retrieved)
	}

	// Update value
	err = store.UpsertLedgerSeq(ctx, source, 2000)
	if err != nil {
		t.Fatalf("failed to update ledger seq: %v", err)
	}

	// Verify updated value
	retrieved, err = store.GetLedgerSeq(ctx, source)
	if err != nil {
		t.Fatalf("failed to get ledger seq: %v", err)
	}

	if retrieved != 2000 {
		t.Errorf("expected ledger_seq 2000, got %d", retrieved)
	}
}

func TestProposalsTable(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	proposals := []*governor.Proposal{
		{
			ProposalKey:     "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC-0",
			ContractId:      "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
			ProposalId:      0,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          0,
			Title:           "Unicorns are real",
			Description:     "They live in the clouds",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       1000,
			VoteEnd:         2000,
			VotesFor:        "0",
			VotesAgainst:    "0",
			VotesAbstain:    "0",
			ExecutionUnlock: 0,
			ExecutionTxHash: "",
		},
		{
			ProposalKey:     "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC-1",
			ContractId:      "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC",
			ProposalId:      1,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          0,
			Title:           "Unicorns are fake",
			Description:     "They are just a myth",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       1100,
			VoteEnd:         2100,
			VotesFor:        "0",
			VotesAgainst:    "0",
			VotesAbstain:    "0",
			ExecutionUnlock: 0,
			ExecutionTxHash: "",
		},
		{
			ProposalKey:     "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB-0",
			ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
			ProposalId:      0,
			Proposer:        "GAQ3OLLBLCO2DZZJHKB2GJNDI445NYNIOP7SMPRDYRUMWWR7YRF2CYVO",
			Status:          1,
			Title:           "Teapot",
			Description:     "Is a teapot",
			Action:          "AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl",
			VoteStart:       400,
			VoteEnd:         800,
			VotesFor:        "1212341314",
			VotesAgainst:    "94895",
			VotesAbstain:    "8234",
			ExecutionUnlock: 12300,
			ExecutionTxHash: "",
		},
	}

	for _, proposal := range proposals {
		err := store.UpsertProposal(ctx, proposal)
		if err != nil {
			t.Fatalf("failed to set proposal: %v", err)
		}
	}

	// Verify get proposal
	retrieved, err := store.GetProposal(ctx, proposals[2].ProposalKey)
	if err != nil {
		t.Fatalf("failed to get proposal: %v", err)
	}
	if diff := cmp.Diff(proposals[2], retrieved); diff != "" {
		t.Errorf("check 1: mismatch (-want +got):\n%s", diff)
	}

	// verify proposal upsert
	newProposal0 := &governor.Proposal{
		ProposalKey:     proposals[0].ProposalKey,
		ContractId:      "bad",
		ProposalId:      99,
		Proposer:        "bad",
		Status:          1,
		Title:           "bad",
		Description:     "bad",
		Action:          "bad",
		VoteStart:       9999,
		VoteEnd:         999,
		VotesFor:        "1000000000000000",
		VotesAgainst:    "5000",
		VotesAbstain:    "2000",
		ExecutionUnlock: 4000,
		ExecutionTxHash: "pretend_tx_hash",
	}
	expectedProposal0 := &governor.Proposal{
		ProposalKey:     proposals[0].ProposalKey,
		ContractId:      proposals[0].ContractId,
		ProposalId:      proposals[0].ProposalId,
		Proposer:        proposals[0].Proposer,
		Status:          newProposal0.Status,
		Title:           proposals[0].Title,
		Description:     proposals[0].Description,
		Action:          proposals[0].Action,
		VoteStart:       proposals[0].VoteStart,
		VoteEnd:         proposals[0].VoteEnd,
		VotesFor:        newProposal0.VotesFor,
		VotesAgainst:    newProposal0.VotesAgainst,
		VotesAbstain:    newProposal0.VotesAbstain,
		ExecutionUnlock: newProposal0.ExecutionUnlock,
		ExecutionTxHash: newProposal0.ExecutionTxHash,
	}
	err = store.UpsertProposal(ctx, newProposal0)
	if err != nil {
		t.Fatalf("failed to set proposal: %v", err)
	}
	retrieved, err = store.GetProposal(ctx, proposals[0].ProposalKey)
	if err != nil {
		t.Fatalf("failed to get proposal after upsert: %v", err)
	}
	if diff := cmp.Diff(expectedProposal0, retrieved); diff != "" {
		t.Errorf("check 2: mismatch (-want +got):\n%s", diff)
	}

	// Verify get proposals by contract id
	retrievedProposals, err := store.GetProposalsByContractId(ctx, proposals[1].ContractId)
	if err != nil {
		t.Fatalf("failed to get proposals by contract id: %v", err)
	}
	if len(retrievedProposals) != 2 {
		t.Fatalf("expected 2 proposals, got %d", len(retrievedProposals))
	}
	if diff := cmp.Diff(proposals[1], retrievedProposals[0]); diff != "" {
		t.Errorf("check 3a: mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(expectedProposal0, retrievedProposals[1]); diff != "" {
		t.Errorf("check 3b: mismatch (-want +got):\n%s", diff)
	}
}

func TestVotesTable(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	contractId := "contract_123"
	proposalId := uint32(1)

	// Insert multiple votes
	votes := []*governor.Vote{
		{
			TxHash:          "tx_vote_001",
			ContractId:      contractId,
			ProposalId:      proposalId,
			Voter:           "user_abc",
			Support:         1,
			Amount:          "1000",
			LedgerSeq:       5000,
			LedgerCloseTime: 1761053046,
		},
		{
			TxHash:          "tx_vote_002",
			ContractId:      contractId,
			ProposalId:      proposalId,
			Voter:           "user_def",
			Support:         2,
			Amount:          "500",
			LedgerSeq:       5100,
			LedgerCloseTime: 1761054046,
		},
		{
			TxHash:          "tx_vote_003",
			ContractId:      contractId,
			ProposalId:      2, // Different proposal
			Voter:           "user_ghi",
			Support:         3,
			Amount:          "750",
			LedgerSeq:       5200,
			LedgerCloseTime: 1761055046,
		},
	}

	for _, vote := range votes {
		if err := store.InsertVote(ctx, vote); err != nil {
			t.Fatalf("failed to insert vote: %v", err)
		}
	}

	// test GetVote
	retrievedVote, err := store.GetVote(ctx, votes[1].TxHash)
	if err != nil {
		t.Fatalf("failed to get vote: %v", err)
	}
	if diff := cmp.Diff(votes[1], retrievedVote); diff != "" {
		t.Errorf("check 1: mismatch (-want +got):\n%s", diff)
	}

	// verify Insert does nothing on duplicate tx_hash
	duplicateVote := &governor.Vote{
		TxHash:          votes[1].TxHash,
		ContractId:      "bad",
		ProposalId:      99,
		Voter:           "bad",
		Support:         0,
		Amount:          "0",
		LedgerSeq:       0,
		LedgerCloseTime: 0,
	}
	if err := store.InsertVote(ctx, duplicateVote); err != nil {
		t.Fatalf("failed to insert duplicate vote: %v", err)
	}
	retrievedVote, err = store.GetVote(ctx, votes[1].TxHash)
	if err != nil {
		t.Fatalf("failed to get vote after duplicate insert: %v", err)
	}
	if diff := cmp.Diff(votes[1], retrievedVote); diff != "" {
		t.Errorf("check 2: mismatch (-want +got):\n%s", diff)
	}

	// test GetVotesByProposal
	retrievedVotes, err := store.GetVotesByProposal(ctx, contractId, proposalId)
	if err != nil {
		t.Fatalf("failed to get votes by proposal: %v", err)
	}
	if len(retrievedVotes) != 2 {
		t.Fatalf("expected 2 votes, got %d", len(retrievedVotes))
	}
	if diff := cmp.Diff(votes[0], retrievedVotes[1]); diff != "" {
		t.Errorf("check 3a: mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(votes[1], retrievedVotes[0]); diff != "" {
		t.Errorf("check 3b: mismatch (-want +got):\n%s", diff)
	}

}
