package governor

import (
	"encoding/json"
	"fmt"
)

type Vote struct {
	TxHash          string
	ContractId      string
	ProposalId      uint32
	Voter           string
	Support         uint32
	Amount          string
	LedgerSeq       uint32
	LedgerCloseTime int64
}

func NewVoteFromVoteCastEvent(event *GovernorEvent) (*Vote, error) {
	if event.EventType != "vote_cast" {
		return nil, fmt.Errorf("invalid event type %s", event.EventType)
	}

	var voteCastData *VoteCastData
	err := json.Unmarshal([]byte(event.EventData), &voteCastData)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal vote_cast event data: %w", err)
	}

	vote := &Vote{
		TxHash:          event.TxHash,
		ContractId:      event.ContractId,
		ProposalId:      event.ProposalId,
		Voter:           voteCastData.Voter,
		Support:         voteCastData.Support,
		Amount:          voteCastData.Amount,
		LedgerSeq:       event.LedgerSeq,
		LedgerCloseTime: event.LedgerCloseTime,
	}
	return vote, nil
}
