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
- Notify a channel whenever something occurs on a Confluence object, we support the following events:
  - space_updated
  - page_created
  - page_restored
  - page_trashed
  - page_updated
  - comment_created
  - comment_updated
- Users can only create subscriptions for the spaces and pages for which they have permission and same for the notifications as well. 
- Admins can change permissions for both Mattermost and Confluence side from the plugin configuration.
    ![image](https://github.com/Brightscout/mattermost-plugin-confluence-fork/assets/90389917/b94fbf84-f345-4ded-aa8d-21de1b870062)

## Configure notifications

- The ``Alias`` (Subscription Name) is intended to be an easy to remember name for the subscription. You will use this name when you need to edit the configuration again. 

- `Confluence Base URL` is the URL of the Confluence server this rule is intended to come from. The Confluence server must have been setup by an administrator with Mattermost using the `/confluence install` command prior to using it in a subscription.

- `Subscribe To` is used to specify if they want to follow events for a Page or a Space object. 

- `Space Key` is the Confluence space key used for the project, often it is 2-4 characters, such as "PROJ" or "MM" and is unique for each Space on that Confluence server.

- `Page ID` is the ID of the Page object on Confuence. Since a page name can be changed by users, the underlying PageID is used to ensure tracking continues even if the page is renamed. The pageID of a Confluence page can be found by going to the "..." menu on the page, then selecting **Page Info**. The URL will then show the PageID in the URL at the end.

    ![image](https://github.com/mattermost/mattermost-plugin-confluence/assets/74422101/9314abd2-8562-456e-9661-7f23c91db206)
    
- `Events` are the internal confluence events that will trigger a notification from Confluence. The following events are currently included:
  - space_updated
  - page_created
  - page_restored
  - page_trashed
  - page_updated
  - comment_created
  - comment_updated
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
