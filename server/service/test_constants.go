package service

import "github.com/mattermost/mattermost-plugin-confluence/server/serializer"

// Test channel IDs (26 characters to match Mattermost format)
const (
	testChannelID1      = "ch1abc1234567890123456789"
	testChannelID2      = "ch2def1234567890123456789"
	testChannelID3      = "ch3ghi1234567890123456789"
	testChannelNotFound = "chnotfound123456789012"
)

// Test subscription aliases
const (
	testAliasSpace1   = "test-space-subscription"
	testAliasSpace2   = "another-space-sub"
	testAliasPage1    = "test-page-subscription"
	testAliasPage2    = "another-page-sub"
	testAliasNotFound = "nonexistent-subscription"
)

// Test URLs and keys
const (
	testBaseURL        = "https://test.confluence.com"
	testSpaceKey1      = "TEST"
	testSpaceKey2      = "TEST2"
	testPageID1        = "PAGE-12345"
	testPageID2        = "PAGE-67890"
	testPageIDNotFound = "PAGE-NOTFOUND"
)

// getBaseTestSubscriptions returns a common set of test subscriptions
func getBaseTestSubscriptions() serializer.Subscriptions {
	return serializer.Subscriptions{
		ByChannelID: map[string]serializer.StringSubscription{
			testChannelID1: {
				testAliasSpace1: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID1,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
			testChannelID2: {
				testAliasPage1: serializer.PageSubscription{
					PageID: testPageID1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasPage1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
					},
				},
				testAliasSpace2: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace2,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
		},
		ByURLSpaceKey: map[string]serializer.StringArrayMap{
			"confluence_subs/test.confluence.com/TEST": {
				testChannelID1: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
		},
		ByURLPageID: map[string]serializer.StringArrayMap{
			"confluence_subs/test.confluence.com/PAGE-12345": {
				testChannelID2: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
		},
	}
}

// getExtendedTestSubscriptions returns a larger set of test subscriptions for more complex scenarios
func getExtendedTestSubscriptions() serializer.Subscriptions {
	return serializer.Subscriptions{
		ByChannelID: map[string]serializer.StringSubscription{
			testChannelID1: {
				testAliasSpace1: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID1,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
				testAliasPage2: serializer.PageSubscription{
					PageID: testPageID2,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasPage2,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID1,
						Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
			testChannelID2: {
				testAliasPage1: serializer.PageSubscription{
					PageID: testPageID1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasPage1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
					},
				},
				testAliasSpace2: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey1,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace2,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID2,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
			testChannelID3: {
				testAliasSpace1: serializer.SpaceSubscription{
					SpaceKey: testSpaceKey2,
					BaseSubscription: serializer.BaseSubscription{
						Alias:     testAliasSpace1,
						BaseURL:   testBaseURL,
						ChannelID: testChannelID3,
						Events:    []string{serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
					},
				},
			},
		},
		ByURLSpaceKey: map[string]serializer.StringArrayMap{
			"confluence_subs/test.confluence.com/TEST": {
				testChannelID1: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
			"confluence_subs/test.confluence.com/TEST2": {
				testChannelID3: {serializer.CommentRemovedEvent, serializer.CommentUpdatedEvent},
			},
		},
		ByURLPageID: map[string]serializer.StringArrayMap{
			"confluence_subs/test.confluence.com/PAGE-12345": {
				testChannelID2: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
			"confluence_subs/test.confluence.com/PAGE-67890": {
				testChannelID1: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
				testChannelID2: {serializer.CommentCreatedEvent, serializer.CommentUpdatedEvent},
			},
		},
	}
}
