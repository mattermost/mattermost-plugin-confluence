{{ .ConfluenceURL }} has been successfully added. To finish the configuration, create a new app in your Confluence instance following these steps:

1. Go to the Atlassian [Developer Console](https://developer.atlassian.com/console/myapps/).
    ![image](https://user-images.githubusercontent.com/90389917/202149604-1f0b8758-96ab-49b0-bd9f-f23ce9d6beda.png)
2. Select **Create > OAuth2.0 integration**.
3. Select **Authorization** in the left menu.
4. Next to OAuth 2.0 (3LO), select **Configure**.
5. In the **Callback URL** field enter:
``
    {{ .RedirectURL }}
``.
6. Select **Permissions** in the left menu.
7. Add the **Confluence API**, and then select `Read user`, `Write Confluence content`, `Read Confluence content all`, and `Read Confluence detailed content` scopes, and select **Save**.
8. Copy the `clientID` and `clientSecret` from **Settings**, and paste them into the modal in mattermost which can be opened by using "/confluence config add" slash command.
9. In Mattermost, use the "/confluence connect" slash command to connect your Mattermost account with your Confluence account.
To finish the configuration, add a new app in your Confluence Cloud instance by following these steps:
1. Go to **Settings > Apps > Manage Apps**.
2. Select **Settings** at the bottom of the page, enable development mode, and apply the change. Development mode allows you to install apps from outside of the Atlassian Marketplace.
3. Select **Upload App**.
4. In the **From this URL** field, enter: `{{ .CloudURL }}`.
5. Once installed, you'll see the "Installed and ready to go!" message.

If you see an option to create a Confluence issue, you're all set! If not, refer to our [documentation](https://mattermost.gitbook.io/plugin-confluence) for troubleshooting help.
