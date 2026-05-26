// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractMentionAccountIDsFromStorage(t *testing.T) {
	for name, tc := range map[string]struct {
		body []byte
		want []string
	}{
		"empty body": {body: nil, want: nil},

		"no mentions": {
			body: []byte(`<p>Just some text.</p>`),
			want: []string{},
		},

		"single account-id mention": {
			body: []byte(`<p>hi <ac:link><ri:user ri:account-id="acct-1"/></ac:link></p>`),
			want: []string{"acct-1"},
		},

		"legacy userkey mention": {
			body: []byte(`<p>hi <ac:link><ri:user ri:userkey="user-key-1"/></ac:link></p>`),
			want: []string{"user-key-1"},
		},

		"multiple mentions across nested macros": {
			body: []byte(`<p>cc
				<ac:link><ri:user ri:account-id="acct-1"/></ac:link>
				and
				<ac:link><ri:user ri:account-id="acct-2"/></ac:link>
			</p>
			<ac:structured-macro ac:name="info"><ac:rich-text-body>
				<p><ac:link><ri:user ri:userkey="user-key-3"/></ac:link></p>
			</ac:rich-text-body></ac:structured-macro>`),
			want: []string{"acct-1", "acct-2", "user-key-3"},
		},

		"deduplicates repeated mentions": {
			body: []byte(`<p>
				<ri:user ri:account-id="acct-1"/>
				<ri:user ri:account-id="acct-1"/>
			</p>`),
			want: []string{"acct-1"},
		},

		"ignores empty attr value": {
			body: []byte(`<p><ri:user ri:account-id=""/></p>`),
			want: []string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := ExtractMentionAccountIDsFromStorage(tc.body)
			require.NoError(t, err)
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			assert.ElementsMatch(t, want, got)
		})
	}
}
