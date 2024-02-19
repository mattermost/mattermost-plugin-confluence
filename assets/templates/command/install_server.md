{{ .ConfluenceURL }} has been successfully added. To finish the configuration, add an Application Link in your Confluence instance following these steps:

1. Go to [**Settings > Applications > Application Links**]({{ .ConfluenceURL }}/plugins/servlet/applinks/listApplicationLinks)
   ![image](https://user-images.githubusercontent.com/90389917/202149868-a3044351-37bc-43c0-9671-aba169706917.png)
2. Select **Create link**.
3. On the **Create Link** screen, select **External Application** and **Incoming** as
   `Application type` and `Direction` respectively. Select **Continue**.
4. On the **Link Applications** screen, set the following values:
**Name**: `Mattermost`
**Redirect URL**: ```{{ .RedirectURL }}```.
**Application Permissions**: `Admin`
Select **Continue**
5. Copy the `clientID` and `clientSecret` from **Settings**, and paste them into the modal in mattermost which can be opened by using "/confluence config add" slash command.
6. In Mattermost, use the "/confluence connect ```{{ .ConfluenceURL }}``` admin" slash command to connect your Mattermost account with your confluence admin account and save the token of the admin to handle admin restricted functions.
7. Use the "/confluence connect" slash command to connect your Mattermost account with your
Confluence account for all other users.

If you see an option to create a Confluence issue, you're all set! If not, refer to our [documentation](https://mattermost.gitbook.io/plugin-confluence) for troubleshooting help.
