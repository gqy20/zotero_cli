package app

import (
	"context"
	"strings"
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
		{name: "all removes limit", req: ItemFindRequest{Options: backend.FindOptions{All: true}}, want: 0},
		{name: "explicit positive limit", req: ItemFindRequest{Options: backend.FindOptions{Limit: 7}}, want: 7},
		{name: "programmatic positive limit", req: ItemFindRequest{Options: backend.FindOptions{Limit: 9}}, want: 9},
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

func TestFindItemsRejectsAllWithLimit(t *testing.T) {
	service := ReadService{}
	_, err := service.FindItems(context.Background(), ItemFindRequest{Options: backend.FindOptions{All: true, Limit: 5}})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("error = %v", err)
	}
}
