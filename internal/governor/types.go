package governor

import (
	"fmt"

	"github.com/stellar/go/amount"
	"github.com/stellar/go/xdr"
)

// The vote counts for a proposal
type VoteCount struct {
	For     string `json:"for"`
	Against string `json:"against"`
	Abstain string `json:"abstain"`
}

func NewVoteCountFromXDR(data xdr.ScVal) (*VoteCount, error) {
	mapData, ok := data.GetMap()
	if !ok {
		return nil, fmt.Errorf("vote_count is not a map")
	}
	var voteCount VoteCount
	for _, entry := range *mapData {
		key, ok := entry.Key.GetSym()
		if !ok {
			return nil, fmt.Errorf("vote_count key is not a symbol")
		}
		switch string(key) {
		case "_for":
			val, ok := entry.Val.GetI128()
			if !ok {
				return nil, fmt.Errorf("vote_count _for is not an i128")
			}
			voteCount.For = amount.String128Raw(val)
		case "against":
			val, ok := entry.Val.GetI128()
			if !ok {
				return nil, fmt.Errorf("vote_count against is not an i128")
			}
			voteCount.Against = amount.String128Raw(val)
		case "abstain":
			val, ok := entry.Val.GetI128()
			if !ok {
				return nil, fmt.Errorf("vote_count abstain is not an i128")
			}
			voteCount.Abstain = amount.String128Raw(val)
		default:
			return nil, fmt.Errorf("unknown vote_count key: %s", string(key))
		}

	}
	if voteCount.For == "" || voteCount.Against == "" || voteCount.Abstain == "" {
		return nil, fmt.Errorf("missing required fields in event data")
	}

	return &voteCount, nil
}
