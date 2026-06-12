// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package main

import "github.com/mattermost/mattermost-plugin-confluence/server/serializer"

// Forge -> internal event names. Forge `deleted:*` collapses onto the existing
// `*_removed` constants so subscriptions made under Connect keep firing.
// Docs: https://developer.atlassian.com/platform/forge/events-reference/confluence/
var forgeToInternalEvent = map[string]string{
	"avi:confluence:created:page":    serializer.PageCreatedEvent,
	"avi:confluence:updated:page":    serializer.PageUpdatedEvent,
	"avi:confluence:trashed:page":    serializer.PageTrashedEvent,
	"avi:confluence:restored:page":   serializer.PageRestoredEvent,
	"avi:confluence:deleted:page":    serializer.PageRemovedEvent,
	"avi:confluence:created:comment": serializer.CommentCreatedEvent,
	"avi:confluence:updated:comment": serializer.CommentUpdatedEvent,
	"avi:confluence:deleted:comment": serializer.CommentRemovedEvent,
}
