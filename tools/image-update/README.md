# image-update

This tool is intended to make updating container images that are used in mwcli much easier.

Data about currently used container images is stored in `data.yml`.

Defined files and directories are scanned for references to the defined container images, they are updated when found.

You can check all images for tags that look newer:

```sh
go run ./tools/image-update/check/check.go
```

If updates are possible `check.go` will output commands that can be used to update the images throughout mwcli.

Those commands will use `update.go`, and look something like this:

```sh
go run ./tools/image-update/update/update.go <old image> <new image>
```

These commands are also written to a file for convenience...

```
tools/image-update/.update.sh
```

In the future the desire would be for this to run in CI and make MRs where easily possible...