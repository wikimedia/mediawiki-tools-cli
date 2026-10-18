# image-update

Check and update container image references used by the development environment.

Data about currently used container images is stored in `data.yml`.

Defined files and directories are scanned for references to the defined container images, they are updated when found.

You can check all images for tags that look newer:

```sh
go run ./tools/image-update/check/check.go
```

If updates are available, `check.go` prints commands to update image references.

Those commands will use `update.go`, and look something like this:

```sh
go run ./tools/image-update/update/update.go <old image> <new image>
```

Multiple pairs of finds and replaces can be passed to `update.go` at once.

```sh
go run ./tools/image-update/update/update.go <old image 1> <new image 1> <old image 2> <new image 2>
```

These commands are also written to a file for convenience...

```
tools/image-update/.update.sh
```

In the future the desire would be for this to run in CI and make MRs where easily possible...