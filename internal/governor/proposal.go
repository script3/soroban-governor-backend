package governor

import (
	"encoding/json"
	"fmt"
)

type Proposal struct {
	ProposalKey     string
	ContractId      string
	ProposalId      uint32
	Proposer        string
	Status          uint32
	Title           string
	Description     string
	Action          string
	VoteStart       uint32
	VoteEnd         uint32
	VotesFor        string
	VotesAgainst    string
	VotesAbstain    string
	ExecutionUnlock uint32
	ExecutionTxHash string
}

// EncodeProposalKey generates a unique key for a proposal based on contractId and proposalId
func EncodeProposalKey(contractId string, proposalId uint32) string {
	return fmt.Sprintf("%s-%d", contractId, proposalId)
}

// NewProposalFromProposalCreatedEvent constructs a Proposal from a GovernorEvent of type "proposal_created"
func NewProposalFromProposalCreatedEvent(event *GovernorEvent) (*Proposal, error) {
	if event.EventType != "proposal_created" {
		return nil, fmt.Errorf("invalid event type %s", event.EventType)
	}

	var proposalCreatedData *ProposalCreatedData
	err := json.Unmarshal([]byte(event.EventData), &proposalCreatedData)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal proposal_created event data: %w", err)
	}

	proposal := &Proposal{
		ProposalKey:     EncodeProposalKey(event.ContractId, event.ProposalId),
		ContractId:      event.ContractId,
		ProposalId:      event.ProposalId,
		Proposer:        proposalCreatedData.Proposer,
		Status:          0,
		Title:           proposalCreatedData.Title,
		Description:     proposalCreatedData.Desc,
		Action:          proposalCreatedData.Action,
		VoteStart:       proposalCreatedData.VoteStart,
		VoteEnd:         proposalCreatedData.VoteEnd,
		VotesFor:        "0",
		VotesAgainst:    "0",
		VotesAbstain:    "0",
		ExecutionUnlock: 0,
		ExecutionTxHash: "",
	}

	return proposal, nil
}
