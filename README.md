# Disclaimer

**This repository is community supported and not maintained by Mattermost. Mattermost disclaims liability for integrations, including Third Party Integrations and Mattermost Integrations. Integrations may be modified or discontinued at any time.**

## Contents

- [Overview](#overview)
- [Features](#features)
- [Admin Guide](/docs/admin-guide.md)
- Contribute
  - [Development](#development)
- [License](/LICENSE)

# Mattermost Plugin for Confluence 

[![Build Status](https://img.shields.io/circleci/project/github/mattermost/mattermost-plugin-confluence/master)](https://circleci.com/gh/mattermost/mattermost-plugin-confluence)
[![Code Coverage](https://img.shields.io/codecov/c/github/mattermost/mattermost-plugin-confluence/master)](https://codecov.io/gh/mattermost/mattermost-plugin-confluence)
[![Release](https://img.shields.io/github/v/release/mattermost/mattermost-plugin-confluence)](https://github.com/mattermost/mattermost-plugin-confluence/releases/latest)
[![HW](https://img.shields.io/github/issues/mattermost/mattermost-plugin-confluence/Up%20For%20Grabs?color=dark%20green&label=Help%20Wanted)](https://github.com/mattermost/mattermost-plugin-confluence/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22Up+For+Grabs%22+label%3A%22Help+Wanted%22)

## Overview

A Mattermost plugin for Confluence. Supports Confluence Cloud, Server, and Data Center versions, as well as environments with multiple Confluence servers. This plugin helps your teams collaborate and keep in sync as Confluence Pages and Spaces get updated. Comments and activity can be pushed into specific Mattermost channels for full visibility. 

## Features

With the Confluence plugin, you can subscribe to a variety of events in Confluence, and specify which channels the associated notifications will appear in. 

- In a Mattermost channel, setup subscriptions to get updates from Confluence and manage your notification subscriptions directly within Mattermost.
- Notify a channel whenever something occurs on a Confluence object:
  - Confluence spaces, including those created, updated, deleted, and restored, and those with added comments.
  - Confluence pages, including those created, updated, deleted, restored, and those with added, deleted, or updated comments.

**Note**: For Confluence server version greater than 9, users need to be connected to Mattermost in order to get the Confluence notifications, or the admin has to setup an API token.

## Configure notifications

- The ``Alias`` (Subscription Name) is intended to be an easy to remember name for the subscription. You will use this name when you need to edit the configuration again. 

- `Confluence Base URL` is the URL of the Confluence server this rule is intended to come from. The Confluence server must have been setup by an administrator with Mattermost using the `/confluence install` command prior to using it in a subscription.

- `Subscribe To` is used to specify if they want to follow events for a Page or a Space object. 

- `Space Key` is the Confluence space key used for the project, often it is 2-4 characters, such as "PROJ" or "MM" and is unique for each Space on that Confluence server.

- `Page ID` is the ID of the Page object on Confuence. Since a page name can be changed by users, the underlying PageID is used to ensure tracking continues even if the page is renamed. The pageID of a Confluence page can be found by going to the "..." menu on the page, then selecting **Page Info**. The URL will then show the PageID in the URL at the end.

    ![image](https://github.com/mattermost/mattermost-plugin-confluence/assets/74422101/9314abd2-8562-456e-9661-7f23c91db206)
    
- `Events` are the internal confluence events that will trigger a notification from Confluence. The following events are currently included:
    - Confluence spaces, including those created, updated, deleted, and restored, and those with added comments.
    - Confluence pages, inlcuidng those created, updated, deleted, restored, and those with added, deleted, or updated comments.

Example of a configured notification:

![image](https://github.com/mattermost/mattermost-plugin-confluence/assets/74422101/33bc67f8-8d36-4e79-a386-7791f4dcd1ee)

### /confluence subscribe

Show a list of all the subscriptions set for the current channel. When you need to see what subscription rules are setup for a channel, you can run `/confluence list` to see a list of the configured subscriptions.

![image](https://github.com/mattermost/mattermost-plugin-confluence/assets/74422101/33c2a456-b7d1-41a2-ba55-53a492a7483c)

### /confluence edit

Edit existing subscription settings. To change the subscription settings, you need to pass over the subscription name to edit. If you have spaces in your subscription names, you need to wrap them in quotation marks. 

For example: `/confluence edit "Project A Subscription"`

To display a list of all the subscription names in the channel, type /confluence list then use the subscription names to edit the correct rule.  

![image](https://github.com/mattermost/mattermost-plugin-confluence/assets/74422101/81ec7a75-b6c1-4513-ad11-763f92416dc8)

### /confluence unsubscribe

Stop receiving notifications to a channel. To stop receiving notifications to a channel, use the `unsubscribe` command to specify the subscription that should be unsubscribed.
example: `/confluence unsubscribe "Project A Subscription"`.

## Development 

This plugin contains both a server and web app portion. Read our documentation about the [Developer Workflow](https://developers.mattermost.com/integrate/plugins/developer-workflow/) and [Developer Setup](https://developers.mattermost.com/integrate/plugins/developer-setup/) for more information about developing and extending plugins.

## Note for Confluence server versions greater or equal to 9

- For Confluence server version greater than or equal to 9, run `/confluence install server` and follow the setup instructions
- User who triggers the event on Confluence must be connected to Mattermost in order to get the notification
- Generic notifications will be received for Page subscriptions if the user who triggers the event on Confluence is not connected to Mattermost
- Administrators can setup an Admin API Token in the plugin configuration to allow notifications for events even when the user who triggers the event on Confluence is not connected to Mattermost

### / confluence connect

Connect your Mattermost account to Confluence.

### / confluence disconnect

Disconnect your Mattermost account to Confluence.

### Releasing new versions

The version of a plugin is determined at compile time, automatically populating a `version` field in the [plugin manifest](plugin.json):
* If the current commit matches a tag, the version will match after stripping any leading `v`, e.g. `1.3.1`.
* Otherwise, the version will combine the nearest tag with `git rev-parse --short HEAD`, e.g. `1.3.1+d06e53e1`.
* If there is no version tag, an empty version will be combined with the short hash, e.g. `0.0.0+76081421`.

To disable this behaviour, manually populate and maintain the `version` field.
