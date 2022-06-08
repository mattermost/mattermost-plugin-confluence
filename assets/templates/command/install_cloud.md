{{ .ConfluenceURL }} has been successfully added. To finish the configuration, create a new app in your Confluence instance following these steps:

1. Navigate to [Developer Console](https://developer.atlassian.com/console/myapps/).
2. Click **Create > OAuth2.0 integration**.
3. Select **Authorization** in the left menu.
4. Next to OAuth 2.0 (3LO), select **Configure**.
5. In the **Callback URL** field enter:
``
    {{ .RedirectURL }}
``.
6. Select **Permissions** in the left menu.
7. Add the **Confluence API** and select `Read user`, `Write Confluence content`, `Read Confluence content all` and `Read Confluence detailed content` scopes and click **Save**.
8. Copy the `clientID` and `clientSecret` from **Settings**  and add them into the plugin configuration.
9. Use the "/confluence connect" command to connect your Mattermost account with your Confluence account.
10. Click the "More Actions" (...) option of any message in the channel (available when you hover over a message).
To finish the configuration, add a new app in your Confluence Cloud instance following these steps:
1. Navigate to **Settings > Apps > Manage Apps**.
2. Choose **Settings** at the bottom of the page, enable development mode, and apply the change. Development mode allows you to install apps from outside of the Atlassian Marketplace.
3. Press **Upload App**.
4. In **From this URL**, enter: `{{ .CloudURL }}`.
5. Once installed, you will see the "Installed and ready to go!" message.

If you see an option to create a Confluence issue, you're all set! If not, refer to our [documentation](https://mattermost.gitbook.io/plugin-confluence) for troubleshooting help.