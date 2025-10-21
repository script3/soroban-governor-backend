package governor

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/strkey"
	"github.com/stellar/go/toid"
	"github.com/stellar/go/xdr"
)

var (
	ErrInvalidEventFormat = errors.New("event format is not valid")
	ErrEventParsingFailed = errors.New("governor event parsing failed")
)

// Construct a unique eventId for an event, using the eventId pattern from the Stellar RPC.
// It combines a 19-character TOID and a 10-character, zero-padded event index, separated by a hyphen
//
// Ref: https://developers.stellar.org/docs/data/apis/rpc/api-reference/methods/getEvents
func EncodeEventId(ledgerSeq uint32, txIndex int32, opIndex int32, eventIndex int32) string {
	// toid package has a TODO to accept uint32. Casting here is OK as we will never reach a ledger > int32::MAX.
	opToid := toid.New(int32(ledgerSeq), txIndex, opIndex).ToInt64()

	opToidString := fmt.Sprintf("%019d", opToid)
	eventIndexString := fmt.Sprintf("%010d", eventIndex)

	return opToidString + "-" + eventIndexString
}

type GovernorEvent struct {
	// Unique identifier for the event
	EventId string
	// StrKey address of the contract emitting the event
	ContractId string
	// Associated proposal ID, if applicable
	ProposalId uint32
	// The event type
	EventType string
	// Additional data payload, JSON encoded
	EventData string
	// Transaction hash that triggered the event
	TxHash string
	// Ledger sequence when the event was emitted
	LedgerSeq uint32
	// Ledger close time (in seconds since epoch) for the ledger the event was emitted
	LedgerCloseTime int64
}

func NewGovernorEventFromContractEvent(ce *xdr.ContractEvent, txHash string, ledgerCloseTime int64, ledgerSeq uint32, txIndex int32, opIndex int32, eventIndex int32) (*GovernorEvent, error) {
	fmt.Printf("Parsing potential governor event\n")
	if ce.Type != xdr.ContractEventTypeContract ||
		ce.ContractId == nil ||
		ce.Body.V != 0 {
		return nil, fmt.Errorf("not contract event: %w", ErrInvalidEventFormat)
	}

	contractId, err := strkey.Encode(strkey.VersionByteContract, ce.ContractId[:])
	if err != nil {
		return nil, fmt.Errorf("unable to encode contractId: %w", ErrEventParsingFailed)
	}

	eventBody, ok := ce.Body.GetV0()
	if !ok {
		return nil, fmt.Errorf("unable to read body: %w", ErrEventParsingFailed)
	}

	fmt.Printf("Event body parsed\n")

	eventId := EncodeEventId(ledgerSeq, txIndex, opIndex, eventIndex)

	fmt.Printf("Event Id parsed %s\n", eventId)

	if len(eventBody.Topics) < 2 {
		return nil, fmt.Errorf("not governor event: %w", ErrInvalidEventFormat)
	}

	// all events have topic[0] = event type and topic[1] = proposal id
	eventTypeXdr, ok := eventBody.Topics[0].GetSym()
	if !ok {
		return nil, fmt.Errorf("not governor event: %w", ErrInvalidEventFormat)
	}
	eventType := string(eventTypeXdr)

	fmt.Printf("Event type parsed %s\n", eventType)

	proposalIdXdr, ok := eventBody.Topics[1].GetU32()
	if !ok {
		return nil, fmt.Errorf("invalid event topic: %w", ErrInvalidEventFormat)
	}
	proposalId := uint32(proposalIdXdr)

	fmt.Printf("Proposal id parsed %d\n", proposalId)

	var eventData string
	switch eventType {
	case "proposal_created":
		proposalCreatedData, err := NewProposalCreatedDataFromEventBody(eventBody)
		if err != nil {
			return nil, err
		}

		dataBytes, err := json.Marshal(proposalCreatedData)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal proposal_created event data: %w", ErrEventParsingFailed)
		}

		eventData = string(dataBytes)
	case "proposal_canceled":
		// no additional data
		eventData = "{}"
	case "proposal_voting_closed":
		votingClosedData, err := NewProposalVotingClosedDataFromEventBody(eventBody)
		if err != nil {
			return nil, err
		}

		dataBytes, err := json.Marshal(votingClosedData)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal proposal_voting_closed event data: %w", ErrEventParsingFailed)
		}

		eventData = string(dataBytes)
	case "proposal_executed":
		// no additional data
		eventData = "{}"
	case "proposal_expired":
		// no additional data
		eventData = "{}"
	case "vote_cast":
		voteCastData, err := NewVoteCastDataFromEventBody(eventBody)
		if err != nil {
			return nil, err
		}

		dataBytes, err := json.Marshal(voteCastData)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal vote_cast event data: %w", ErrEventParsingFailed)
		}

		eventData = string(dataBytes)
	default:
		return nil, fmt.Errorf("invalid event type %s: %w", eventType, ErrInvalidEventFormat)
	}

	fmt.Printf("Proposal data parsed %s\n", eventData)

	ge := GovernorEvent{
		EventId:         eventId,
		ContractId:      contractId,
		ProposalId:      proposalId,
		EventType:       eventType,
		EventData:       eventData,
		TxHash:          txHash,
		LedgerSeq:       ledgerSeq,
		LedgerCloseTime: ledgerCloseTime,
	}
	return &ge, nil
}

// Event data emitted when a proposal is created
type ProposalCreatedData struct {
	// Address of the proposer
	Proposer string `json:"proposer"`
	// Title of the proposal
	Title string `json:"title"`
	// Description of the proposal
	Desc string `json:"desc"`
	// Action to be executed if the proposal passes, as a base64-encoded XDR string
	Action string `json:"action"`
	// Ledger sequence when voting starts
	VoteStart uint32 `json:"vote_start"`
	// Ledger sequence when voting ends
	VoteEnd uint32 `json:"vote_end"`
}

func NewProposalCreatedDataFromEventBody(body xdr.ContractEventV0) (*ProposalCreatedData, error) {
	if len(body.Topics) != 3 {
		return nil, fmt.Errorf("unexpected number of topics in event: %w", ErrInvalidEventFormat)
	}

	proposerXdr, ok := body.Topics[2].GetAddress()
	if !ok {
		return nil, fmt.Errorf("invalid proposer in event topic: %w", ErrInvalidEventFormat)
	}
	proposer := proposerXdr.AccountId.Address()

	vecData, ok := body.Data.GetVec()
	if !ok {
		return nil, fmt.Errorf("event data is not a vec %w", ErrInvalidEventFormat)
	}
	if len(*vecData) != 5 {
		return nil, fmt.Errorf("unexpected number of fields in event data: %w", ErrInvalidEventFormat)
	}

	var data ProposalCreatedData
	data.Proposer = proposer
	for i, entry := range *vecData {
		switch i {
		case 0:
			val, ok := entry.GetStr()
			if !ok {
				return nil, fmt.Errorf("title is not a str %w", ErrEventParsingFailed)
			}
			data.Title = string(val)
		case 1:
			val, ok := entry.GetStr()
			if !ok {
				return nil, fmt.Errorf("desc is not a str  %w", ErrEventParsingFailed)
			}
			data.Desc = string(val)
		case 2:
			byteStr, err := entry.MarshalBinary()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal action data %w", ErrEventParsingFailed)
			}
			data.Action = base64.StdEncoding.EncodeToString(byteStr)
		case 3:
			val, ok := entry.GetU32()
			if !ok {
				return nil, fmt.Errorf("vote_start is not a u32 %w", ErrEventParsingFailed)
			}
			data.VoteStart = uint32(val)
		case 4:
			val, ok := entry.GetU32()
			if !ok {
				return nil, fmt.Errorf("vote_end is not a u32 %w", ErrEventParsingFailed)
			}
			data.VoteEnd = uint32(val)
		default:
			return nil, fmt.Errorf("too many entries %d %w", i, ErrEventParsingFailed)
		}
	}
	return &data, nil
}

// Event data emitted when voting on a proposal is closed
type ProposalVotingClosedData struct {
	// Final status of the proposal
	Status uint32 `json:"status"`
	// Eta ledger sequence when the proposal can be executed, if applicable
	Eta uint32 `json:"eta"`
	// The final vote counts
	FinalVotes VoteCount `json:"final_votes"`
}

func NewProposalVotingClosedDataFromEventBody(body xdr.ContractEventV0) (*ProposalVotingClosedData, error) {
	if len(body.Topics) != 4 {
		return nil, fmt.Errorf("unexpected number of topics %d in event: %w", len(body.Topics), ErrInvalidEventFormat)
	}

	status, ok := body.Topics[2].GetU32()
	if !ok {
		return nil, fmt.Errorf("invalid event topic: %w", ErrInvalidEventFormat)
	}

	eta, ok := body.Topics[3].GetU32()
	if !ok {
		return nil, fmt.Errorf("invalid event topic: %w", ErrInvalidEventFormat)
	}

	finalVotes, err := NewVoteCountFromXDR(body.Data)
	if err != nil {
		return nil, fmt.Errorf("unable to parse final votes: %w", ErrEventParsingFailed)
	}
	data := ProposalVotingClosedData{
		Status:     uint32(status),
		Eta:        uint32(eta),
		FinalVotes: *finalVotes,
	}
	return &data, nil
}

// Event emitted when a vote is cast on a proposal
type VoteCastData struct {
	// Address of the voter
	Voter string `json:"voter"`
	// Vote option selected
	Support uint32 `json:"support"`
	// Vote count
	Amount string `json:"amount"`
}

func NewVoteCastDataFromEventBody(body xdr.ContractEventV0) (*VoteCastData, error) {
	if len(body.Topics) != 3 {
		return nil, fmt.Errorf("unexpected number of topics in event: %w", ErrInvalidEventFormat)
	}

	voterXdr, ok := body.Topics[2].GetAddress()
	if !ok {
		return nil, fmt.Errorf("invalid proposer in event topic: %w", ErrInvalidEventFormat)
	}
	voter := voterXdr.AccountId.Address()

	vecData, ok := body.Data.GetVec()
	if !ok {
		return nil, fmt.Errorf("event data is not a vec %w", ErrInvalidEventFormat)
	}
	if len(*vecData) != 2 {
		return nil, fmt.Errorf("unexpected number of fields in event data: %w", ErrInvalidEventFormat)
	}

	var data VoteCastData
	data.Voter = voter
	for i, entry := range *vecData {
		switch i {
		case 0:
			val, ok := entry.GetU32()
			if !ok {
				return nil, fmt.Errorf("support is not a u32 %w", ErrEventParsingFailed)
			}
			data.Support = uint32(val)
		case 1:
			val, ok := entry.GetI128()
			if !ok {
				return nil, fmt.Errorf("amount is not an i128 %w", ErrEventParsingFailed)
			}
			data.Amount = amount.String128Raw(val)
		default:
			return nil, fmt.Errorf("too many entries %d %w", i, ErrEventParsingFailed)
		}
	}
	return &data, nil
}
