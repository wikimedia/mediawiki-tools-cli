# image-update

Check and update container image references used by the development environment.

Data about currently used container images is stored in `data.yml`.

Defined files and directories are scanned for references to the defined container images, they are updated when found.

You can check all images for tags that look newer:

```sh
go run ./tools/image-update/check/check.go
```

If updates are available, `check.go` prints commands to update image references. It reads Docker Hub, Wikimedia and Quay tag listings, follows registry pagination, and selects the highest matching version. `requireRegex` matches the whole tag; use it to preserve a release family or runtime version. Shellbox variants must share a build timestamp.

Failed registry checks are reported separately and return a nonzero exit code. Generated command files are refreshed on every check so old proposals are not reused. Review the proposed changes before applying them; this tool does not validate application compatibility.

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

The weekly GitLab pipeline uses the generated matrix to open image update MRs.
