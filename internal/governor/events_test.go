package governor

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stellar/go-stellar-sdk/xdr"
)

func TestEncodeEventId(t *testing.T) {
	tests := []struct {
		name       string
		opToid     int64
		eventIndex int32
		want       string
	}{
		{
			name:       "all zeros",
			opToid:     0,
			eventIndex: 0,
			want:       "0000000000000000000-0000000000",
		},
		{
			name:       "correctly creates event id",
			opToid:     4752467212378112,
			eventIndex: 999999,
			want:       "0004752467212378112-0000999999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodeEventId(tt.opToid, tt.eventIndex)
			if got != tt.want {
				t.Errorf("\nResult = %v\nWant = %v\n", got, tt.want)
			}
		})
	}
}

func TestNewGovernorEventFromContractEvent(t *testing.T) {
	tests := []struct {
		name            string
		eventXdr        string
		txHash          string
		ledgerCloseTime int64
		ledgerSeq       uint32
		opToid          int64
		eventIndex      int32
		want            *GovernorEvent
	}{
		{
			name:            "proposal_created_calldata",
			eventXdr:        "AAAAAAAAAAHA70OsAU+gdeyDov6bvqWGNPZnEemjXsRPq/7W4n00/AAAAAEAAAAAAAAAAwAAAA8AAAAQcHJvcG9zYWxfY3JlYXRlZAAAAAMAAAADAAAAEgAAAAAAAAAALJ/M6wbqSvh6BcSe5KJD8aWHCTFHGu3YUKtUqAH05uUAAAAQAAAAAQAAAAUAAAAOAAAAGE1ha2UgbWUgc2VjdXJpdHkgY291bmNpbAAAAA4AAAADcGx6AAAAABAAAAABAAAAAgAAAA8AAAAHQ291bmNpbAAAAAASAAAAAAAAAAAsn8zrBupK+HoFxJ7kokPxpYcJMUca7dhQq1SoAfTm5QAAAAMAEa9sAAAAAwAR8uw=",
			txHash:          "cb759f7b061992ac79e5f944a08238a24d2999a5ac58eee9fde35dff6404d970",
			ledgerCloseTime: 1761053041,
			ledgerSeq:       1170134,
			opToid:          5025687261941760,
			eventIndex:      0,
			want: &GovernorEvent{
				EventId:         "0005025687261941760-0000000000",
				ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
				EventType:       "proposal_created",
				ProposalId:      3,
				EventData:       `{"proposer":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","title":"Make me security council","desc":"plz","action":"AAAAEAAAAAEAAAACAAAADwAAAAdDb3VuY2lsAAAAABIAAAAAAAAAACyfzOsG6kr4egXEnuSiQ/GlhwkxRxrt2FCrVKgB9Obl","vote_start":1159020,"vote_end":1176300}`,
				TxHash:          "cb759f7b061992ac79e5f944a08238a24d2999a5ac58eee9fde35dff6404d970",
				LedgerSeq:       1170134,
				LedgerCloseTime: 1761053041,
			},
		},
		{
			name:            "proposal_canceled",
			eventXdr:        "AAAAAAAAAAHA70OsAU+gdeyDov6bvqWGNPZnEemjXsRPq/7W4n00/AAAAAEAAAAAAAAAAgAAAA8AAAARcHJvcG9zYWxfY2FuY2VsZWQAAAAAAAADAAAAAwAAAAE=",
			txHash:          "cb759f7b061992ac79e5f944a08238a24d2999a5ac58eee9fde35dff6404d970",
			ledgerCloseTime: 1761053046,
			ledgerSeq:       1170136,
			opToid:          5025695851872256,
			eventIndex:      0,
			want: &GovernorEvent{
				EventId:         "0005025695851872256-0000000000",
				ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
				EventType:       "proposal_canceled",
				ProposalId:      3,
				EventData:       `{}`,
				TxHash:          "cb759f7b061992ac79e5f944a08238a24d2999a5ac58eee9fde35dff6404d970",
				LedgerSeq:       1170136,
				LedgerCloseTime: 1761053046,
			},
		},
		{
			name:            "vote_cast",
			eventXdr:        "AAAAAAAAAAHA70OsAU+gdeyDov6bvqWGNPZnEemjXsRPq/7W4n00/AAAAAEAAAAAAAAAAwAAAA8AAAAJdm90ZV9jYXN0AAAAAAAAAwAAAAIAAAASAAAAAAAAAAAsn8zrBupK+HoFxJ7kokPxpYcJMUca7dhQq1SoAfTm5QAAABAAAAABAAAAAgAAAAMAAAAAAAAACgAAAAAAAAAAAAAABKgXyAA=",
			txHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
			ledgerCloseTime: 1761053046,
			ledgerSeq:       1170136,
			opToid:          5025695851876451,
			eventIndex:      42,
			want: &GovernorEvent{
				EventId:         "0005025695851876451-0000000042",
				ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
				EventType:       "vote_cast",
				ProposalId:      2,
				EventData:       `{"voter":"GAWJ7THLA3VEV6D2AXCJ5ZFCIPY2LBYJGFDRV3OYKCVVJKAB6TTOLZ5Q","support":0,"amount":"20000000000"}`,
				TxHash:          "caa081584805c84f4e74b904b201fe765c16f7e3ed784d87e8dd531c621c62db",
				LedgerSeq:       1170136,
				LedgerCloseTime: 1761053046,
			},
		},
		{
			name:            "proposal_voting_closed",
			eventXdr:        "AAAAAAAAAAHA70OsAU+gdeyDov6bvqWGNPZnEemjXsRPq/7W4n00/AAAAAEAAAAAAAAABAAAAA8AAAAWcHJvcG9zYWxfdm90aW5nX2Nsb3NlZAAAAAAAAwAAAAEAAAADAAAAAgAAAAMAAAAAAAAAEQAAAAEAAAADAAAADwAAAARfZm9yAAAACgAAAAAAAAAAAAAAAElQT4AAAAAPAAAAB2Fic3RhaW4AAAAACgAAAAAAAAAAAAAAAAAAAAAAAAAPAAAAB2FnYWluc3QAAAAACgAAAAAAAAAAAAAABKgXyAA=",
			txHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
			ledgerCloseTime: 1761053050,
			ledgerSeq:       1170137,
			opToid:          5025700146839602,
			eventIndex:      3,
			want: &GovernorEvent{
				EventId:         "0005025700146839602-0000000003",
				ContractId:      "CDAO6Q5MAFH2A5PMQORP5G56UWDDJ5THCHU2GXWEJ6V75VXCPU2PZYPB",
				EventType:       "proposal_voting_closed",
				ProposalId:      1,
				EventData:       `{"status":2,"eta":0,"final_votes":{"for":"1230000000","against":"20000000000","abstain":"0"}}`,
				TxHash:          "e65cfb5071126dc0a21b9d77f6d26a9d5788edf1cb6aac8de6e478273c1957f5",
				LedgerSeq:       1170137,
				LedgerCloseTime: 1761053050,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ce xdr.ContractEvent
			err := xdr.SafeUnmarshalBase64(tt.eventXdr, &ce)
			if err != nil {
				t.Fatalf("Setup Failed: Unable to unmarshal contract event xdr: %v", err)
			}
			got, err := NewGovernorEventFromContractEvent(&ce, tt.txHash, tt.ledgerSeq, tt.ledgerCloseTime, tt.opToid, tt.eventIndex)
			if err != nil {
				t.Fatalf("returned error: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
