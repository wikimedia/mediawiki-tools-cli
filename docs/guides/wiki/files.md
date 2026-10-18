# Upload files and new versions

Use `mw wiki file upload` with a local image, recording or other file accepted by the destination wiki. For Commons, use WebM for video and include a description, author, source and free license. Uploads are manual; generating documentation or recordings does not publish them.

## Authenticate

Create a bot password at the destination wiki's `Special:BotPasswords`. Enable **Edit existing pages**, **Create, edit, and move pages**, **Upload new files**, and **Upload, replace, and move files**. Uploading also needs the page-editing permissions because it creates a file description page. The bot password can only exercise rights your account already has.

Save the password in a private temporary file. In Bash, this keeps it out of shell history and command arguments:

```sh
password_file=$(mktemp)
read -r -s -p 'Bot password: ' wiki_password
printf '\n'
printf '%s' "$wiki_password" > "$password_file"
unset wiki_password
```

Pass the bot login name (for example `YourName@upload`) with `--user` and the file with `--password-file`. The CLI logs in for each invocation; it does not remember wiki credentials. When finished, remove it with `rm -- "$password_file"` and `unset password_file`. Existing wiki commands also accept `--password-file`.

## Upload a file

Write the initial description and license as wikitext in `description.wiki`. For Commons, use its Information template and the license template appropriate to your media. Choose a descriptive, stable wiki filename:

```sh
mw wiki file upload demo.webm --wiki commons.wikimedia.org --user 'YourName@upload' --password-file "$password_file" --filename 'A Wikimedia CLI - Guide - tools-kubernetes-pod-helloworld - en.webm' --text-file description.wiki --comment 'Record the Toolforge pod guide'
```

Without `--filename`, the local basename is used. `--text` accepts inline wikitext instead of `--text-file`. `--dry-run` shows the source and destination without logging in or contacting the wiki. Check new upload commands against `--wiki test.wikipedia.org` first; this is a separate destination, not a preview of an upload to Commons.

## Upload a new version

Use the same wiki filename with `--replace`:

```sh
mw wiki file upload demo.webm --wiki commons.wikimedia.org --user 'YourName@upload' --password-file "$password_file" --filename 'A Wikimedia CLI - Guide - tools-kubernetes-pod-helloworld - en.webm' --replace --comment 'Refresh the walkthrough'
```

This adds a file revision and preserves the existing description. `--replace` allows a replacement but also permits a new file if that name is not present. Without it, an existing-file warning stops publication. Other warnings, such as duplicate content, still stop the command; review them before explicitly accepting them with `--ignore-warnings`. That option alone does not authorize replacing an existing file.

Commons permits minor corrections under the same filename, while substantially different demonstrations should have a new dated filename. Mark a recording intended to be updated with `{{Current}}` and follow [Commons' overwrite guidance](https://commons.wikimedia.org/wiki/Commons:Overwriting_existing_files).
