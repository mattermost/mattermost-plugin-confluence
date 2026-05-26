// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package service

import (
	"encoding/json"

	"github.com/mattermost/mattermost-plugin-confluence/server/util"
)

type adfNode struct {
	Type    string                 `json:"type"`
	Attrs   map[string]interface{} `json:"attrs,omitempty"`
	Content []adfNode              `json:"content,omitempty"`
	Marks   []adfNode              `json:"marks,omitempty"`
}

func ExtractMentionAccountIDsFromADF(body []byte) ([]string, error) {
	if len(body) == 0 {
		return nil, nil
	}

	var root adfNode
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}

	var ids []string
	walkADF(&root, &ids)
	return util.Deduplicate(ids), nil
}

func walkADF(n *adfNode, ids *[]string) {
	if n == nil {
		return
	}
	if n.Type == "mention" {
		if id := mentionAccountID(n.Attrs); id != "" {
			*ids = append(*ids, id)
		}
	}
	for i := range n.Content {
		walkADF(&n.Content[i], ids)
	}
}

func mentionAccountID(attrs map[string]interface{}) string {
	if attrs == nil {
		return ""
	}
	// Skip bot mentions; no human user behind them.
	if ut, ok := attrs["userType"].(string); ok && ut == "APP" {
		return ""
	}
	id, _ := attrs["id"].(string)
	return id
}
