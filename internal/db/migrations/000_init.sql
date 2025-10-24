-- Create history table for storing governor events
-- ref /internal/governor/events.go: GovernorEvent
CREATE TABLE IF NOT EXISTS history (
    event_id TEXT PRIMARY KEY,
    contract_id TEXT NOT NULL,
    proposal_id INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    event_data TEXT NOT NULL,
    tx_hash TEXT NOT NULL,
    ledger_seq INTEGER NOT NULL,
    ledger_close_time BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_history_contract_ledger ON history(contract_id, ledger_seq DESC);

-- Create proposals table to storing governor proposals
-- ref /internal/governor/proposals.go: GovernorProposal
CREATE TABLE IF NOT EXISTS proposals (
    proposal_key TEXT PRIMARY KEY,
    contract_id TEXT NOT NULL,
    proposal_id INTEGER NOT NULL,
    proposer TEXT NOT NULL,
    status INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    action TEXT NOT NULL,
    vote_start BIGINT NOT NULL,
    vote_end BIGINT NOT NULL,
    votes_for TEXT NOT NULL,
    votes_against TEXT NOT NULL,
    votes_abstain TEXT NOT NULL,
    execution_unlock INTEGER NOT NULL,
    execution_tx_hash TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_proposals_contract_proposal_id ON proposals(contract_id, proposal_id DESC);

-- Create votes table to track votes on proposals
-- ref /internal/governor/votes.go: GovernorVote
CREATE TABLE IF NOT EXISTS votes (
    tx_hash TEXT PRIMARY KEY,
    contract_id TEXT NOT NULL,
    proposal_id INTEGER NOT NULL,
    voter TEXT NOT NULL,
    support INTEGER NOT NULL,
    amount TEXT NOT NULL,
    ledger_seq INTEGER NOT NULL,
    ledger_close_time BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_votes_contract_proposal ON votes(contract_id, proposal_id);

-- Create status table to track processed ledgers
CREATE TABLE IF NOT EXISTS status (
    source TEXT PRIMARY KEY,
    ledger_seq INTEGER NOT NULL,
    ledger_close_time BIGINT NOT NULL
);
