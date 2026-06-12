// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMentionAccountIDsFromADF(t *testing.T) {
	for name, tc := range map[string]struct {
		body []byte
		want []string
	}{
		"empty body returns nil": {
			body: nil,
			want: nil,
		},
		"document without mentions returns empty": {
			body: []byte(`{
				"type":"doc","version":1,
				"content":[{"type":"paragraph","content":[{"type":"text","text":"hello"}]}]
			}`),
			want: []string{},
		},
		"single inline mention": {
			body: []byte(`{
				"type":"doc","version":1,
				"content":[{"type":"paragraph","content":[
					{"type":"text","text":"hey "},
					{"type":"mention","attrs":{"id":"acct-1","text":"@Alice"}}
				]}]
			}`),
			want: []string{"acct-1"},
		},
		"deduplicates repeats": {
			body: []byte(`{
				"type":"doc",
				"content":[
					{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"acct-1"}}]},
					{"type":"paragraph","content":[{"type":"mention","attrs":{"id":"acct-1"}}]}
				]
			}`),
			want: []string{"acct-1"},
		},
		"multiple distinct mentions across nested nodes": {
			body: []byte(`{
				"type":"doc",
				"content":[
					{"type":"bulletList","content":[
						{"type":"listItem","content":[
							{"type":"paragraph","content":[
								{"type":"mention","attrs":{"id":"acct-1"}},
								{"type":"text","text":" and "},
								{"type":"mention","attrs":{"id":"acct-2"}}
							]}
						]},
						{"type":"listItem","content":[
							{"type":"paragraph","content":[
								{"type":"mention","attrs":{"id":"acct-3"}}
							]}
						]}
					]}
				]
			}`),
			want: []string{"acct-1", "acct-2", "acct-3"},
		},
		"skips APP (bot) user mentions": {
			body: []byte(`{
				"type":"doc",
				"content":[{"type":"paragraph","content":[
					{"type":"mention","attrs":{"id":"bot-1","userType":"APP"}},
					{"type":"mention","attrs":{"id":"acct-1","userType":"DEFAULT"}}
				]}]
			}`),
			want: []string{"acct-1"},
		},
		"ignores mention nodes missing id": {
			body: []byte(`{
				"type":"doc",
				"content":[{"type":"paragraph","content":[
					{"type":"mention","attrs":{"text":"@orphan"}},
					{"type":"mention","attrs":{"id":"acct-1"}}
				]}]
			}`),
			want: []string{"acct-1"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractMentionAccountIDsFromADF(tc.body)
			require.NoError(t, err)
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			assert.ElementsMatch(t, want, got)
		})
	}
}

func TestExtractMentionAccountIDsFromADF_InvalidJSON(t *testing.T) {
	_, err := ExtractMentionAccountIDsFromADF([]byte(`{not json`))
	assert.Error(t, err)
}
