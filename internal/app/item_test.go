package app

import (
	"testing"

	"zotero_cli/internal/backend"
)

func TestFindResultLimitUsesOutputWeightAndExplicitOverrides(t *testing.T) {
	tests := []struct {
		name string
		req  ItemFindRequest
		want int
	}{
		{name: "plain", req: ItemFindRequest{}, want: 100},
		{name: "fulltext lean", req: ItemFindRequest{Options: backend.FindOptions{In: "fulltext"}}, want: 100},
		{name: "snippet", req: ItemFindRequest{Snippet: true}, want: 20},
		{name: "full item", req: ItemFindRequest{Options: backend.FindOptions{Full: true}}, want: 20},
		{name: "implicit all from filters stays bounded", req: ItemFindRequest{Options: backend.FindOptions{All: true}}, want: 100},
		{name: "explicit all", req: ItemFindRequest{Options: backend.FindOptions{All: true}, ExplicitAll: true}, want: 0},
		{name: "explicit positive limit", req: ItemFindRequest{Options: backend.FindOptions{Limit: 7}}, want: 7},
		{name: "programmatic positive limit", req: ItemFindRequest{Options: backend.FindOptions{Limit: 9}}, want: 9},
		{name: "limit wins over all", req: ItemFindRequest{Options: backend.FindOptions{All: true, Limit: 5}, ExplicitAll: true}, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := backend.NormalizeFindOptions(tt.req.Options)
			if got := findResultLimit(tt.req, opts); got != tt.want {
				t.Fatalf("findResultLimit() = %d, want %d", got, tt.want)
			}
		})
	}
}
