// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"regexp"

	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

// Regex over XML parser: Confluence storage format embeds macro syntax that
// trips strict XML parsers.
var (
	riUserAccountIDRE = regexp.MustCompile(`<ri:user[^>]*\sri:account-id\s*=\s*"([^"]+)"`)
	riUserKeyRE       = regexp.MustCompile(`<ri:user[^>]*\sri:userkey\s*=\s*"([^"]+)"`)
)

func ExtractMentionAccountIDsFromStorage(body []byte) ([]string, error) {
	if len(body) == 0 {
		return nil, nil
	}

	var ids []string
	for _, re := range []*regexp.Regexp{riUserAccountIDRE, riUserKeyRE} {
		for _, m := range re.FindAllSubmatch(body, -1) {
			if len(m) > 1 && len(m[1]) > 0 {
				ids = append(ids, string(m[1]))
			}
		}
	}
	return util.Deduplicate(ids), nil
}
