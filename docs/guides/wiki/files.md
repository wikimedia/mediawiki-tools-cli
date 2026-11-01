# Download, convert and upload files

## Download an original or converted image

Pass a file-description URL to download from its wiki, or a wiki filename with
`--wiki`. Public downloads do not need credentials. The default format,
`original`, saves the uploaded bytes unchanged:

```sh
mw wiki file download 'https://commons.wikimedia.org/wiki/File:Logo_of_a-wikimedia-cli.svg'
```

For a genuine PNG, choose a format and optionally a width:

```sh
mw wiki file download 'https://commons.wikimedia.org/wiki/File:Logo_of_a-wikimedia-cli.svg' --format png --width 1024 --output logo.png
```

MediaWiki renders SVGs and supplies a preview; the CLI decodes the returned image
and encodes PNG or JPEG locally. This also handles servers returning WebP despite
a `.png` thumbnail URL. No external image-conversion program is needed. PNG keeps
transparency; JPEG uses a white background. Conversion produces a single still
image, not an animation or video conversion.

`--width` gives the exact output width and preserves the aspect ratio. Without
it, converted images use the source width (1024 pixels if unknown). Large images
may need a smaller width. Without `--output`, the wiki filename gets the selected
extension. Existing files require `--overwrite`; `--output -` writes binary bytes
to standard output. Private wikis can use the same authentication flags as
uploads below.

## Upload files and new versions

Use `mw wiki file upload` with a local image, recording or other file accepted by the destination wiki. For Commons, use WebM for video and include a description, author, source and free license. Uploads are manual; generating documentation or recordings does not publish them.

### Authenticate

For Wikimedia sites, authorize a saved profile with the built-in OAuth client:

```sh
mw wiki auth login --profile wikimedia/uploads --scope basic,editpage,createeditmovepage,uploadfile,uploadeditmovefile
```

Approve access in the browser. The profile works across Wikimedia sites, including
Commons. Basic-only access cannot upload files; add `--replace` to the login
command when updating an existing profile's authorization.

For another wiki, create a bot password at its `Special:BotPasswords`. Enable
**Edit existing pages**, **Create, edit, and move pages**, **Upload new files**,
and **Upload, replace, and move files**, then save it as a separate profile:

```sh
mw wiki auth login --profile example/uploads --wiki https://wiki.example/w/api.php --method password --username 'YourName@upload'
```

Enter the bot password at the hidden prompt. Credentials are saved using your
selected [credential provider](../authentication.md#saved-wiki-profiles); uploads
reuse the profile. For that wiki, substitute `--profile example/uploads` and its
`--wiki` endpoint in the examples below.

Uploads need page-editing permissions to create the file description page. OAuth
and bot passwords can only exercise rights your account already has.

### Upload a file

Write the initial description and license as wikitext in `description.wiki`. For Commons, use its Information template and the license template appropriate to your media. Choose a descriptive, stable wiki filename:

```sh
mw wiki file upload demo.webm --wiki commons.wikimedia.org --profile wikimedia/uploads --filename 'A Wikimedia CLI - Guide - tools-kubernetes-pod-helloworld - en.webm' --text-file description.wiki --comment 'Record the Toolforge pod guide'
```

Without `--filename`, the local basename is used. `--text` accepts inline wikitext instead of `--text-file`. `--dry-run` shows the source and destination without logging in or contacting the wiki. Check new upload commands against `--wiki test.wikipedia.org` first; this is a separate destination, not a preview of an upload to Commons.

### Upload a new version

Use the same wiki filename with `--replace`:

```sh
mw wiki file upload demo.webm --wiki commons.wikimedia.org --profile wikimedia/uploads --filename 'A Wikimedia CLI - Guide - tools-kubernetes-pod-helloworld - en.webm' --replace --comment 'Refresh the walkthrough'
```

This adds a file revision and preserves the existing description. `--replace` allows a replacement but also permits a new file if that name is not present. Without it, an existing-file warning stops publication. Other warnings, such as duplicate content, still stop the command; review them before explicitly accepting them with `--ignore-warnings`. That option alone does not authorize replacing an existing file.

Commons permits minor corrections under the same filename, while substantially different demonstrations should have a new dated filename. Mark a recording intended to be updated with `{{Current}}` and follow [Commons' overwrite guidance](https://commons.wikimedia.org/wiki/Commons:Overwriting_existing_files).
